package remote

import (
	"context"
	"errors"
	"io"
	"os"
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
	var f *os.File
	var err error
	if e.Root != nil {
		f, err = e.Root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	} else {
		f, err = os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	}
	if err != nil {
		return Result{}, errors.New("cannot create destination: choose a new file inside the allowed directory")
	}
	result, callErr := e.Client.Call(ctx, op, args, f)
	closeErr := f.Close()
	if err := errors.Join(callErr, closeErr); err != nil {
		if e.Root != nil {
			_ = e.Root.Remove(name)
		} else {
			_ = os.Remove(name)
		}
		return Result{}, err
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
