package files

import (
	"io/fs"
	"mime"
	"path"
	"strings"
	"time"
)

type Kind string

const (
	KindFile Kind = "file"
	KindDir  Kind = "dir"
)

type Category string

const (
	CategoryImage    Category = "image"
	CategoryVideo    Category = "video"
	CategoryAudio    Category = "audio"
	CategoryDocument Category = "document"
	CategoryArchive  Category = "archive"
	CategoryOther    Category = "other"
)

type Entry struct {
	Path    Path
	Kind    Kind
	Size    int64
	ModTime time.Time
}

func (e Entry) Name() string {
	return e.Path.Name()
}

func (e Entry) ContentType() string {
	if e.Kind == KindDir {
		return ""
	}
	return ContentTypeOf(e.Name())
}

func newEntry(p Path, info fs.FileInfo) Entry {
	entry := Entry{Path: p, Kind: KindFile, Size: info.Size(), ModTime: info.ModTime().UTC()}
	if info.IsDir() {
		entry.Kind = KindDir
		entry.Size = 0
	}
	return entry
}

var _extraTypes = map[string]string{
	".txt":  "text/plain; charset=utf-8",
	".md":   "text/markdown; charset=utf-8",
	".csv":  "text/csv; charset=utf-8",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".avif": "image/avif",
	".heic": "image/heic",
	".svg":  "image/svg+xml",
	".mp4":  "video/mp4",
	".m4v":  "video/mp4",
	".webm": "video/webm",
	".mkv":  "video/x-matroska",
	".mov":  "video/quicktime",
	".avi":  "video/x-msvideo",
	".mp3":  "audio/mpeg",
	".m4a":  "audio/mp4",
	".aac":  "audio/aac",
	".wav":  "audio/wav",
	".ogg":  "audio/ogg",
	".flac": "audio/flac",
	".pdf":  "application/pdf",
	".epub": "application/epub+zip",
	".zip":  "application/zip",
	".7z":   "application/x-7z-compressed",
	".zst":  "application/zstd",
}

func ContentTypeOf(name string) string {
	ext := strings.ToLower(path.Ext(name))
	if t, ok := _extraTypes[ext]; ok {
		return t
	}
	if t := mime.TypeByExtension(ext); t != "" {
		return t
	}
	return "application/octet-stream"
}

var _documentExts = map[string]struct{}{
	".pdf": {}, ".doc": {}, ".docx": {}, ".xls": {}, ".xlsx": {}, ".ppt": {}, ".pptx": {},
	".odt": {}, ".ods": {}, ".txt": {}, ".md": {}, ".rtf": {}, ".csv": {}, ".epub": {},
	".pages": {}, ".numbers": {}, ".key": {}, ".json": {}, ".xml": {}, ".html": {},
}

var _archiveExts = map[string]struct{}{
	".zip": {}, ".tar": {}, ".gz": {}, ".tgz": {}, ".bz2": {}, ".xz": {}, ".7z": {},
	".rar": {}, ".zst": {}, ".iso": {}, ".dmg": {},
}

func CategoryOf(name string) Category {
	ext := strings.ToLower(path.Ext(name))
	if _, ok := _archiveExts[ext]; ok {
		return CategoryArchive
	}
	if _, ok := _documentExts[ext]; ok {
		return CategoryDocument
	}
	switch t := ContentTypeOf(name); {
	case strings.HasPrefix(t, "image/"):
		return CategoryImage
	case strings.HasPrefix(t, "video/"):
		return CategoryVideo
	case strings.HasPrefix(t, "audio/"):
		return CategoryAudio
	default:
		return CategoryOther
	}
}

func isReserved(p Path) bool {
	first, _, _ := strings.Cut(p.String(), "/")
	return first == legacyTus
}
