package httpapi

import (
	"time"

	"github.com/din9xtr/cloud-control-panel/internal/auth"
	"github.com/din9xtr/cloud-control-panel/internal/files"
	"github.com/din9xtr/cloud-control-panel/internal/share"
)

type userDTO struct {
	ID                int64     `json:"id"`
	Username          string    `json:"username"`
	PasswordChangedAt time.Time `json:"passwordChangedAt"`
}

type meDTO struct {
	User userDTO `json:"user"`
}

func toMe(u auth.User) meDTO {
	return meDTO{User: userDTO{ID: u.ID, Username: u.Username, PasswordChangedAt: u.PasswordChangedAt}}
}

type sessionDTO struct {
	ID         string    `json:"id"`
	Current    bool      `json:"current"`
	CreatedAt  time.Time `json:"createdAt"`
	LastSeenAt time.Time `json:"lastSeenAt"`
	IP         string    `json:"ip"`
	UserAgent  string    `json:"userAgent"`
}

type itemsDTO[T any] struct {
	Items []T `json:"items"`
}

func newItems[T any](items []T) itemsDTO[T] {
	if items == nil {
		items = []T{}
	}
	return itemsDTO[T]{Items: items}
}

type entryLinkDTO struct {
	ID        string    `json:"id"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type entryDTO struct {
	Name        string        `json:"name"`
	Path        string        `json:"path"`
	Kind        files.Kind    `json:"kind"`
	Size        int64         `json:"size"`
	ModifiedAt  time.Time     `json:"modifiedAt"`
	ContentType string        `json:"contentType,omitempty"`
	Link        *entryLinkDTO `json:"link,omitempty"`
}

func toEntry(e files.Entry, links map[string]share.Link) entryDTO {
	dto := entryDTO{
		Name:        e.Name(),
		Path:        e.Path.String(),
		Kind:        e.Kind,
		Size:        e.Size,
		ModifiedAt:  e.ModTime,
		ContentType: e.ContentType(),
	}
	if l, ok := links[dto.Path]; ok {
		dto.Link = &entryLinkDTO{ID: l.ID, ExpiresAt: l.ExpiresAt}
	}
	return dto
}

func toEntries(entries []files.Entry, links map[string]share.Link) []entryDTO {
	out := make([]entryDTO, 0, len(entries))
	for _, e := range entries {
		out = append(out, toEntry(e, links))
	}
	return out
}

type folderDTO struct {
	Path    string     `json:"path"`
	Entries []entryDTO `json:"entries"`
}

type entryListDTO struct {
	Entries []entryDTO `json:"entries"`
}

type pathDTO struct {
	Path string `json:"path"`
}

type trashItemDTO struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	OriginalPath string     `json:"originalPath"`
	Kind         files.Kind `json:"kind"`
	Size         int64      `json:"size"`
	TrashedAt    time.Time  `json:"trashedAt"`
	PurgeAt      time.Time  `json:"purgeAt"`
}

type linkDTO struct {
	Mode      share.Mode `json:"mode"`
	Kind      files.Kind `json:"kind"`
	ID        string     `json:"id"`
	Path      string     `json:"path"`
	Name      string     `json:"name"`
	URL       string     `json:"url"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt time.Time  `json:"expiresAt"`
	Downloads int64      `json:"downloads"`
}

func toLink(l share.Link) linkDTO {
	url := "/s/" + l.Token
	if l.Kind == files.KindDir {
		url = "/share/" + l.Token
	}
	return linkDTO{
		Mode: l.Mode, Kind: l.Kind,
		ID:        l.ID,
		Path:      l.Path.String(),
		Name:      l.Path.Name(),
		URL:       url,
		CreatedAt: l.CreatedAt,
		ExpiresAt: l.ExpiresAt,
		Downloads: l.Downloads,
	}
}

type diskDTO struct {
	TotalBytes int64 `json:"totalBytes"`
	FreeBytes  int64 `json:"freeBytes"`
}

type folderUsageDTO struct {
	Name  string `json:"name"`
	Bytes int64  `json:"bytes"`
}

type categoryUsageDTO struct {
	Category files.Category `json:"category"`
	Bytes    int64          `json:"bytes"`
}

type largestDTO struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type storageDTO struct {
	QuotaBytes     int64              `json:"quotaBytes"`
	UsedBytes      int64              `json:"usedBytes"`
	FilesBytes     int64              `json:"filesBytes"`
	TrashBytes     int64              `json:"trashBytes"`
	ReservedBytes  int64              `json:"reservedBytes"`
	AvailableBytes int64              `json:"availableBytes"`
	FileCount      int                `json:"fileCount"`
	Disk           diskDTO            `json:"disk"`
	ByFolder       []folderUsageDTO   `json:"byFolder"`
	ByCategory     []categoryUsageDTO `json:"byCategory"`
	Largest        []largestDTO       `json:"largest"`
}

type settingsDTO struct {
	QuotaBytes            int64 `json:"quotaBytes"`
	TrashRetentionDays    int   `json:"trashRetentionDays"`
	DefaultLinkTTLSeconds int64 `json:"defaultLinkTtlSeconds"`
}
