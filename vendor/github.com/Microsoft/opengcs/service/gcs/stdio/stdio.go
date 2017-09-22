package stdio

import (
	"io"
	"os"
	"sync"

	"github.com/Microsoft/opengcs/service/gcs/transport"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ConnectionSet is a structure defining the readers and writers the Core
// implementation should forward a process's stdio through.
type ConnectionSet struct {
	In, Out, Err transport.Connection
}

// Close closes each stdio connection.
func (s *ConnectionSet) Close() error {
	var err error
	if s.In != nil {
		if cerr := s.In.Close(); cerr != nil && err == nil {
			err = errors.Wrap(cerr, "failed Close on stdin")
		}
		s.In = nil
	}
	if s.Out != nil {
		if cerr := s.Out.Close(); cerr != nil && err == nil {
			err = errors.Wrap(cerr, "failed Close on stdout")
		}
		s.Out = nil
	}
	if s.Err != nil {
		if cerr := s.Err.Close(); cerr != nil && err == nil {
			err = errors.Wrap(cerr, "failed Close on stderr")
		}
		s.Err = nil
	}
	return err
}

// FileSet contains os.File fields for stdio.
type FileSet struct {
	In, Out, Err *os.File
}

// Close closes all the FileSet handles.
func (fs *FileSet) Close() error {
	var err error
	if fs.In != nil {
		if cerr := fs.In.Close(); cerr != nil && err == nil {
			err = errors.Wrap(cerr, "failed Close on stdin")
		}
		fs.In = nil
	}
	if fs.Out != nil {
		if cerr := fs.Out.Close(); cerr != nil && err == nil {
			err = errors.Wrap(cerr, "failed Close on stdout")
		}
		fs.Out = nil
	}
	if fs.Err != nil {
		if cerr := fs.Err.Close(); cerr != nil && err == nil {
			err = errors.Wrap(cerr, "failed Close on stderr")
		}
		fs.Err = nil
	}
	return err
}

// Files returns a FileSet with an os.File for each connection
// in the connection set.
func (s *ConnectionSet) Files() (_ *FileSet, err error) {
	fs := &FileSet{}
	defer func() {
		if err != nil {
			fs.Close()
		}
	}()
	if s.In != nil {
		fs.In, err = s.In.File()
		if err != nil {
			return nil, errors.Wrap(err, "failed to dup stdin socket for command")
		}
	}
	if s.Out != nil {
		fs.Out, err = s.Out.File()
		if err != nil {
			return nil, errors.Wrap(err, "failed to dup stdout socket for command")
		}
	}
	if s.Err != nil {
		fs.Err, err = s.Err.File()
		if err != nil {
			return nil, errors.Wrap(err, "failed to dup stderr socket for command")
		}
	}
	return fs, nil
}

// NewPipeRelay returns a new pipe relay wrapping the given connection stdin, stdout, stderr set.
func (s *ConnectionSet) NewPipeRelay() (_ *PipeRelay, err error) {
	pr := &PipeRelay{s: s}
	defer func() {
		if err != nil {
			pr.closePipes()
		}
	}()

	if s.In != nil {
		pr.pipes[0], pr.pipes[1], err = os.Pipe()
		if err != nil {
			return nil, errors.Wrap(err, "failed to create stdin pipe relay")
		}
	}
	if s.Out != nil {
		pr.pipes[2], pr.pipes[3], err = os.Pipe()
		if err != nil {
			return nil, errors.Wrap(err, "failed to create stdout pipe relay")
		}
	}
	if s.Err != nil {
		pr.pipes[4], pr.pipes[5], err = os.Pipe()
		if err != nil {
			return nil, errors.Wrap(err, "failed to create stderr pipe relay")
		}
	}
	return pr, nil
}

// PipeRelay is a relay built to expose a pipe interface
// for stdin, stdout, stderr on top of a ConnectionSet.
type PipeRelay struct {
	wg sync.WaitGroup
	s  *ConnectionSet
	// pipes format is stdin [0 read, 1 write], stdout [2 read, 3 write], stderr [4 read, 5 write].
	pipes [6]*os.File
}

// Files returns a FileSet with an os.File for each connection
// in the connection set.
func (pr *PipeRelay) Files() (*FileSet, error) {
	fs := new(FileSet)
	if pr.s.In != nil {
		fs.In = pr.pipes[0]
	}
	if pr.s.Out != nil {
		fs.Out = pr.pipes[3]
	}
	if pr.s.Err != nil {
		fs.Err = pr.pipes[5]
	}
	return fs, nil
}

// Start starts the relay operation. The caller must call Wait to wait
// for the relay to finish and release the associated resources.
func (pr *PipeRelay) Start() {
	if pr.s.In != nil {
		pr.wg.Add(1)
		go func() {
			if _, err := io.Copy(pr.pipes[1], pr.s.In); err != nil {
				logrus.Errorf("error copying stdin to pipe: %s", err)
			}
			if err := pr.pipes[1].Close(); err != nil {
				logrus.Errorf("error closing stdin write pipe: %s", err)
			}
			pr.pipes[1] = nil
			pr.wg.Done()
		}()
	}
	if pr.s.Out != nil {
		pr.wg.Add(1)
		go func() {
			if _, err := io.Copy(pr.s.Out, pr.pipes[2]); err != nil {
				logrus.Errorf("error copying stdout from pipe: %s", err)
			}
			if err := pr.s.Out.Close(); err != nil {
				logrus.Errorf("error closing stdout socket: %s", err)
			}
			pr.wg.Done()
		}()
	}
	if pr.s.Err != nil {
		pr.wg.Add(1)
		go func() {
			if _, err := io.Copy(pr.s.Err, pr.pipes[4]); err != nil {
				logrus.Errorf("error copying stderr from pipe: %s", err)
			}
			if err := pr.s.Err.Close(); err != nil {
				logrus.Errorf("error closing stderr socket: %s", err)
			}
			pr.wg.Done()
		}()
	}
}

// Wait waits for the relaying to finish and closes the associated
// pipes and connections.
func (pr *PipeRelay) Wait() {
	// Close stdin so that the copying goroutine is safely unblocked; this is necessary
	// because the host expects stdin to be closed before it will report process
	// exit back to the client, and the client expects the process notification before
	// it will close its side of stdin (which io.Copy is waiting on in the copying goroutine).
	if pr.s.In != nil {
		if err := pr.s.In.CloseRead(); err != nil {
			logrus.Errorf("error closing read for stdin: %s", err)
		}
	}

	pr.wg.Wait()
	pr.closePipes()
	pr.s.Close()
}

func (pr *PipeRelay) closePipes() {
	for i := 0; i < len(pr.pipes); i++ {
		if pr.pipes[i] != nil {
			if err := pr.pipes[i].Close(); err != nil {
				logrus.Errorf("failed to close relay pipe: (%s)", err)
			}
			pr.pipes[i] = nil
		}
	}
}

// NewTtyRelay returns a new TTY relay for a given master PTY file.
func (s *ConnectionSet) NewTtyRelay(pty *os.File) *TtyRelay {
	return &TtyRelay{s: s, pty: pty}
}

// TtyRelay relays IO between a set of stdio connections and a master PTY file.
type TtyRelay struct {
	m      sync.Mutex
	closed bool
	wg     sync.WaitGroup
	s      *ConnectionSet
	pty    *os.File
}

// ResizeConsole sends the appropriate resize to a pTTY FD
func (r *TtyRelay) ResizeConsole(height, width uint16) error {
	r.m.Lock()
	defer r.m.Unlock()

	if r.closed {
		return errors.New("error resizing console pty is closed")
	}
	return ResizeConsole(r.pty, height, width)
}

// Start starts the relay operation. The caller must call Wait to wait
// for the relay to finish and release the associated resources.
func (r *TtyRelay) Start() {
	if r.s.In != nil {
		r.wg.Add(1)
		go func() {
			_, err := io.Copy(r.pty, r.s.In)
			if err != nil {
				logrus.Errorf("error copying stdin to pty: %s", err)
			}
			r.wg.Done()
		}()
	}
	if r.s.Out != nil {
		r.wg.Add(1)
		go func() {
			_, err := io.Copy(r.s.Out, r.pty)
			if err != nil {
				logrus.Errorf("error copying pty to stdout: %s", err)
			}
			r.wg.Done()
		}()
	}
}

// Wait waits for the relaying to finish and closes the associated
// files and connections.
func (r *TtyRelay) Wait() {
	// Close stdin so that the copying goroutine is safely unblocked; this is necessary
	// because the host expects stdin to be closed before it will report process
	// exit back to the client, and the client expects the process notification before
	// it will close its side of stdin (which io.Copy is waiting on in the copying goroutine).
	if r.s.In != nil {
		if err := r.s.In.CloseRead(); err != nil {
			logrus.Errorf("error closing read for stdin: %s", err)
		}
	}

	// Wait for all users of stdioSet and master to finish before closing them.
	r.wg.Wait()

	r.m.Lock()
	defer r.m.Unlock()

	r.pty.Close()
	r.closed = true
	r.s.Close()
}
