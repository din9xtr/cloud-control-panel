package httpapi

import (
	"cmp"
	"math"
	"net/http"
	"slices"
	"time"

	"github.com/din9xtr/cloud-control-panel/internal/files"
	"github.com/din9xtr/cloud-control-panel/internal/settings"
	"github.com/din9xtr/cloud-control-panel/internal/share"
)

const day = 24 * time.Hour

func (s *server) listTrash(w http.ResponseWriter, r *http.Request) {
	items, err := s.files.TrashItems(r.Context(), principalFrom(r.Context()).UserID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	retention, err := s.settings.TrashRetention(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := make([]trashItemDTO, 0, len(items))
	for _, item := range items {
		out = append(out, trashItemDTO{
			ID:           item.ID,
			Name:         item.Name(),
			OriginalPath: item.OriginalPath.String(),
			Kind:         item.Kind,
			Size:         item.Size,
			TrashedAt:    item.TrashedAt,
			PurgeAt:      item.TrashedAt.Add(retention),
		})
	}
	writeJSON(w, http.StatusOK, newItems(out))
}

func (s *server) emptyTrash(w http.ResponseWriter, r *http.Request) {
	if err := s.files.EmptyTrash(r.Context(), principalFrom(r.Context()).UserID); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) deleteTrashItem(w http.ResponseWriter, r *http.Request) {
	if err := s.files.DeleteForever(r.Context(), principalFrom(r.Context()).UserID, r.PathValue("id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) restoreTrashItem(w http.ResponseWriter, r *http.Request) {
	p, err := s.files.Restore(r.Context(), principalFrom(r.Context()).UserID, r.PathValue("id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, pathDTO{Path: p.String()})
}

func (s *server) listLinks(w http.ResponseWriter, r *http.Request) {
	links, err := s.shares.List(r.Context(), principalFrom(r.Context()).UserID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := make([]linkDTO, 0, len(links))
	for _, l := range links {
		out = append(out, toLink(l))
	}
	writeJSON(w, http.StatusOK, newItems(out))
}

type createLinkRequest struct {
	Mode       share.Mode `json:"mode"`
	Path       string     `json:"path"`
	TTLSeconds int64      `json:"ttlSeconds"`
}

func (s *server) createLink(w http.ResponseWriter, r *http.Request) {
	var req createLinkRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.fail(w, r, err)
		return
	}
	p, err := files.ParsePath(req.Path)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if req.TTLSeconds < 0 || req.TTLSeconds > int64(share.MaxTTL/time.Second) {
		s.fail(w, r, share.ErrInvalidTTL)
		return
	}
	link, err := s.shares.CreateWithMode(r.Context(), principalFrom(r.Context()).UserID, p, time.Duration(req.TTLSeconds)*time.Second, req.Mode)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toLink(link))
}

func (s *server) revokeLink(w http.ResponseWriter, r *http.Request) {
	if err := s.shares.Revoke(r.Context(), principalFrom(r.Context()).UserID, r.PathValue("id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) storageUsage(w http.ResponseWriter, r *http.Request) {
	userID := principalFrom(r.Context()).UserID
	usage, err := s.files.Usage(r.Context(), userID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	capacity, err := s.files.Capacity(r.Context(), userID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	reserved, err := s.uploads.Reserved(r.Context(), userID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toStorage(usage, capacity, reserved))
}

func toStorage(usage files.Usage, capacity files.Capacity, reserved int64) storageDTO {
	available := capacity.Disk.FreeBytes
	if capacity.QuotaBytes > 0 {
		available = min(available, max(0, capacity.QuotaBytes-usage.UsedBytes()-reserved))
	}
	dto := storageDTO{
		QuotaBytes:     capacity.QuotaBytes,
		UsedBytes:      usage.UsedBytes(),
		FilesBytes:     usage.FilesBytes,
		TrashBytes:     usage.TrashBytes,
		ReservedBytes:  reserved,
		AvailableBytes: available,
		FileCount:      usage.FileCount,
		Disk:           diskDTO{TotalBytes: capacity.Disk.TotalBytes, FreeBytes: capacity.Disk.FreeBytes},
		ByFolder:       make([]folderUsageDTO, 0, len(usage.ByFolder)),
		ByCategory:     make([]categoryUsageDTO, 0, len(usage.ByCategory)),
		Largest:        make([]largestDTO, 0, len(usage.Largest)),
	}
	for _, f := range usage.ByFolder {
		dto.ByFolder = append(dto.ByFolder, folderUsageDTO(f))
	}
	for category, bytes := range usage.ByCategory {
		dto.ByCategory = append(dto.ByCategory, categoryUsageDTO{Category: category, Bytes: bytes})
	}
	slices.SortFunc(dto.ByCategory, func(a, b categoryUsageDTO) int {
		return cmp.Or(cmp.Compare(b.Bytes, a.Bytes), cmp.Compare(a.Category, b.Category))
	})
	for _, e := range usage.Largest {
		dto.Largest = append(dto.Largest, largestDTO{Name: e.Name(), Path: e.Path.String(), Size: e.Size})
	}
	return dto
}

func (s *server) getSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.settings.Get(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toSettingsDTO(cfg))
}

func (s *server) updateSettings(w http.ResponseWriter, r *http.Request) {
	var req settingsDTO
	if err := decodeJSON(w, r, &req); err != nil {
		s.fail(w, r, err)
		return
	}
	retention, ok := durationOf(int64(req.TrashRetentionDays), day)
	if !ok {
		s.fail(w, r, settings.ErrInvalidTrashRetention)
		return
	}
	linkTTL, ok := durationOf(req.DefaultLinkTTLSeconds, time.Second)
	if !ok {
		s.fail(w, r, settings.ErrInvalidLinkTTL)
		return
	}
	cfg, err := s.settings.Update(r.Context(), settings.Settings{
		QuotaBytes:     req.QuotaBytes,
		TrashRetention: retention,
		DefaultLinkTTL: linkTTL,
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toSettingsDTO(cfg))
}

func durationOf(n int64, unit time.Duration) (time.Duration, bool) {
	if n < 0 || n > int64(math.MaxInt64/unit) {
		return 0, false
	}
	return time.Duration(n) * unit, true
}

func toSettingsDTO(cfg settings.Settings) settingsDTO {
	return settingsDTO{
		QuotaBytes:            cfg.QuotaBytes,
		TrashRetentionDays:    int(cfg.TrashRetention / day),
		DefaultLinkTTLSeconds: int64(cfg.DefaultLinkTTL / time.Second),
	}
}
