package remote

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
)

type Executor struct {
	Client     *Client
	Root       *os.Root
	Restricted bool
	Output     io.Writer
	Progress   io.Writer
}

func (e *Executor) Execute(ctx context.Context, op Operation, args map[string]any) (Result, error) {
	if err := op.Validate(args); err != nil {
		return Result{}, err
	}
	if op.Name == "uploadFolder" {
		return e.uploadFolder(ctx, args)
	}
	if op.Name == "uploadFile" {
		f, err := e.openInput(stringValue(args["input"]))
		if err != nil {
			return Result{}, err
		}
		defer f.Close()
		return e.Client.Upload(ctx, f, args, e.Progress)
	}
	if !op.Binary {
		return e.Client.Call(ctx, op, args, nil)
	}
	name := stringValue(args["output"])
	if name == "-" && !e.Restricted {
		return e.Client.Call(ctx, op, args, e.Output)
	}
	if e.Restricted && e.Root == nil {
		return Result{}, errors.New("file transfers require --transfer-dir")
	}
	temp := filepath.Join(filepath.Dir(name), ".cloud-download-"+rand.Text()+".part")
	var f *os.File
	var err error
	if e.Root != nil {
		f, err = e.Root.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	} else {
		f, err = os.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	}
	if err != nil {
		return Result{}, errors.New("cannot create destination: choose a new file inside the allowed directory")
	}
	defer func() {
		if e.Root != nil {
			_ = e.Root.Remove(temp)
		} else {
			_ = os.Remove(temp)
		}
	}()
	result, callErr := e.Client.Call(ctx, op, args, f)
	syncErr := f.Sync()
	closeErr := f.Close()
	if callErr != nil {
		return Result{}, callErr
	}
	if syncErr != nil || closeErr != nil {
		return Result{}, errors.New("cannot save completed download")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if e.Root != nil {
		err = e.Root.Link(temp, name)
	} else {
		err = os.Link(temp, name)
	}
	if err != nil {
		return Result{}, errors.New("cannot publish download: choose a new file inside the allowed directory")
	}
	return result, nil
}

func (e *Executor) openInput(name string) (*os.File, error) {
	if e.Restricted && e.Root == nil {
		return nil, errors.New("file transfers require --transfer-dir")
	}
	var f *os.File
	var err error
	if e.Root != nil {
		f, err = e.Root.Open(name)
	} else {
		f, err = os.Open(name)
	}
	if err != nil {
		return nil, errors.New("cannot read source file inside the allowed directory")
	}
	return f, nil
}
