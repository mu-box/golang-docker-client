package docker

import (
	"io"

	"github.com/docker/docker/pkg/stdcopy"
	dockType "github.com/docker/engine-api/types"
	"github.com/jcelliott/lumber"
	"golang.org/x/net/context"
)

func ExecStart(containerID string, user string, cmd []string, stdIn, stdOut, stdErr, tty bool) (dockType.ContainerExecCreateResponse, dockType.HijackedResponse, error) {
	config := dockType.ExecConfig{
		Tty:          tty,
		User:         user,
		Cmd:          cmd,
		AttachStdin:  stdIn,
		AttachStdout: stdOut,
		AttachStderr: stdErr,
	}

	exec, err := client.ContainerExecCreate(context.Background(), containerID, config)
	if err != nil {
		return exec, dockType.HijackedResponse{}, err
	}
	resp, err := client.ContainerExecAttach(context.Background(), exec.ID, config)
	return exec, resp, err
}

func ExecInspect(id string) (dockType.ContainerExecInspect, error) {
	return client.ContainerExecInspect(context.Background(), id)
}

func ExecPipe(resp dockType.HijackedResponse, inStream io.Reader, outStream, errorStream io.Writer) error {
	var err error
	receiveStdout := make(chan error, 1)
	if outStream != nil || errorStream != nil {
		go func() {
			// always do this because we are never tty
			_, err = stdcopy.StdCopy(outStream, errorStream, resp.Reader)
			lumber.Trace("[hijack] End of stdout")
			receiveStdout <- err
		}()
	}

	stdinDone := make(chan struct{})
	go func() {
		if inStream != nil {
			io.Copy(resp.Conn, inStream)
			lumber.Trace("[hijack] End of stdin")
		}

		if err := resp.CloseWrite(); err != nil {
			lumber.Error("Couldn't send EOF: %s", err)
		}
		close(stdinDone)
	}()

	select {
	case err := <-receiveStdout:
		if err != nil {
			lumber.Debug("Error receiveStdout: %s", err)
			return err
		}
	case <-stdinDone:
		if outStream != nil || errorStream != nil {
			if err := <-receiveStdout; err != nil {
				lumber.Debug("Error receiveStdout: %s", err)
				return err
			}
		}
	}

	return nil
}

// resize the exec.
func ContainerExecResize(id string, height, width int) error {
	return client.ContainerExecResize(context.Background(), id, dockType.ResizeOptions{height, width})
}
