package remote

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
)

func (e *Executor) uploadFolder(ctx context.Context, args map[string]any) (Result, error) {
	if e.Restricted && e.Root == nil {
		return Result{}, errors.New("file transfers require --transfer-dir")
	}
	input := stringValue(args["input"])
	var root *os.Root
	var err error
	var localName string
	if e.Root != nil {
		root, err = e.Root.OpenRoot(input)
		localName = filepath.Join(e.Root.Name(), input)
	} else {
		root, err = os.OpenRoot(input)
		localName, _ = filepath.Abs(input)
	}
	if err != nil {
		return Result{}, errors.New("cannot read source folder inside the allowed directory")
	}
	defer root.Close()
	name := stringValue(args["name"])
	if name == "" {
		name = filepath.Base(filepath.Clean(localName))
	}
	if name == "." || name == ".." || name == "" || name == "/" {
		return Result{}, errors.New("specify a remote folder name")
	}
	destination := path.Join(stringValue(args["folder"]), name)
	ops, err := Catalog()
	if err != nil {
		return Result{}, err
	}
	var create, list Operation
	for _, op := range ops {
		switch op.Name {
		case "createFolder":
			create = op
		case "listFolder":
			list = op
		}
	}
	var filesCount, foldersCount, totalBytes int64
	err = fs.WalkDir(root.FS(), ".", func(relative string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return errors.New("read source folder")
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		target := path.Join(destination, relative)
		if entry.IsDir() {
			if _, err := e.Client.Call(ctx, create, map[string]any{"path": target}, nil); err != nil {
				var apiErr *APIError
				if !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict {
					return err
				}
				if _, err := e.Client.Call(ctx, list, map[string]any{"path": target}, nil); err != nil {
					return err
				}
			}
			foldersCount++
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		file, err := root.Open(relative)
		if err != nil {
			return errors.New("read source file inside folder")
		}
		result, uploadErr := e.Client.Upload(ctx, file, map[string]any{"folder": path.Dir(target), "name": entry.Name(), "conflict": "skip-identical"}, e.Progress)
		closeErr := file.Close()
		if err := errors.Join(uploadErr, closeErr); err != nil {
			return err
		}
		data, ok := result.Data.(map[string]any)
		if !ok {
			return errors.New("invalid upload result")
		}
		count, ok := data["bytes"].(int64)
		if !ok {
			return errors.New("invalid upload byte count")
		}
		totalBytes += count
		filesCount++
		return nil
	})
	if err != nil {
		return Result{}, fmt.Errorf("folder upload stopped after %d files; completed files and folders remain: %w", filesCount, err)
	}
	return Result{Status: http.StatusCreated, Data: map[string]any{"path": destination, "files": filesCount, "folders": foldersCount, "bytes": totalBytes}}, nil
}
