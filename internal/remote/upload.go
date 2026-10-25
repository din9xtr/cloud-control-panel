package remote

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func (c *Client) Upload(ctx context.Context, source *os.File, args map[string]any, progress io.Writer) (Result, error) {
	info, err := source.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return Result{}, errors.New("upload source must be a readable regular file")
	}
	endpoint := "/uploads"
	if token := stringValue(args["token"]); token != "" {
		if len(token) != 22 || !validID(token) {
			return Result{}, errors.New("invalid upload link token")
		}
		if stringValue(args["folder"]) != "" {
			return Result{}, errors.New("upload links determine the destination; omit folder")
		}
		endpoint = "/public/links/" + token + "/uploads"
	}
	name := stringValue(args["name"])
	if name == "" {
		name = info.Name()
	}
	id := stringValue(args["uploadId"])
	if id == "" {
		metadata := "filename " + base64.StdEncoding.EncodeToString([]byte(name)) + ",folder " + base64.StdEncoding.EncodeToString([]byte(stringValue(args["folder"])))
		headers := http.Header{"Tus-Resumable": {"1.0.0"}, "Upload-Length": {strconv.FormatInt(info.Size(), 10)}, "Upload-Metadata": {metadata}}
		resp, err := c.request(ctx, http.MethodPost, endpoint, nil, headers, nil)
		if err != nil {
			return Result{}, err
		}
		_ = resp.Body.Close()
		location := resp.Header.Get("Location")
		prefix := "/api/v1" + endpoint + "/"
		if !strings.HasPrefix(location, prefix) {
			return Result{}, errors.New("invalid upload location")
		}
		id = strings.TrimPrefix(location, prefix)
	}
	if !validID(id) {
		return Result{}, errors.New("invalid upload ID")
	}
	if info.Size() == 0 && stringValue(args["uploadId"]) == "" {
		return Result{Status: http.StatusCreated, Data: map[string]any{"uploadId": id, "bytes": int64(0)}}, nil
	}
	if progress != nil {
		if _, err := fmt.Fprintln(progress, "uploadId:", id); err != nil {
			return Result{}, fmt.Errorf("report upload ID: %w", err)
		}
	}
	result, err := c.resume(ctx, source, endpoint, id, info.Size(), progress)
	if err != nil {
		return Result{}, fmt.Errorf("upload interrupted; resume unchanged source with uploadId %s: %w", id, err)
	}
	return result, nil
}

func (c *Client) resume(ctx context.Context, source *os.File, endpoint, id string, size int64, progress io.Writer) (Result, error) {
	route := endpoint + "/" + id
	headers := http.Header{"Tus-Resumable": {"1.0.0"}}
	resp, err := c.request(ctx, http.MethodHead, route, nil, headers, nil)
	if err != nil {
		return Result{}, err
	}
	_ = resp.Body.Close()
	offset, err := headerInt(resp.Header, "Upload-Offset")
	if err != nil {
		return Result{}, err
	}
	length, err := headerInt(resp.Header, "Upload-Length")
	if err != nil {
		return Result{}, err
	}
	if length != size || offset > size {
		return Result{}, errors.New("upload size differs from source")
	}
	if _, err := source.Seek(offset, io.SeekStart); err != nil {
		return Result{}, errors.New("seek upload source")
	}
	for offset < size || size == 0 {
		chunk := min(int64(4<<20), size-offset)
		headers.Set("Content-Type", "application/offset+octet-stream")
		headers.Set("Upload-Offset", strconv.FormatInt(offset, 10))
		resp, err := c.request(ctx, http.MethodPatch, route, nil, headers, io.LimitReader(source, chunk))
		if err != nil {
			return Result{}, err
		}
		_ = resp.Body.Close()
		next, err := headerInt(resp.Header, "Upload-Offset")
		if err != nil {
			return Result{}, err
		}
		if next != offset+chunk {
			return Result{}, errors.New("unexpected upload offset")
		}
		offset = next
		if progress != nil {
			if _, err := fmt.Fprintf(progress, "%d/%d bytes\n", offset, size); err != nil {
				return Result{}, fmt.Errorf("report upload progress: %w", err)
			}
		}
		if size == 0 {
			break
		}
	}
	return Result{Status: http.StatusNoContent, Data: map[string]any{"uploadId": id, "bytes": offset}}, nil
}
