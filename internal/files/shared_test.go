package files

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSharedFolderCannotFollowSymlinksOutsideItsRoot(t *testing.T) {
	f := newFixture(t, 0)
	f.write(t, "garden/seeds.txt", 4)
	f.write(t, "private/notes.txt", 4)
	base := filepath.Join(f.dir, "users", "1", "garden")
	for name, target := range map[string]string{"shortcut": "../private", "notes.txt": "../private/notes.txt"} {
		if err := os.Symlink(target, filepath.Join(base, name)); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	root := mustPath(t, "garden")
	if entries, err := f.svc.ListShared(ctx, testUser, root, Root); err != nil || len(entries) != 1 || entries[0].Path.String() != "seeds.txt" {
		t.Fatalf("shared entries = %v, %v", entries, err)
	}
	if _, err := f.svc.ListShared(ctx, testUser, root, mustPath(t, "shortcut")); err == nil {
		t.Fatal("listed folder outside shared root")
	}
	for _, name := range []string{"notes.txt", "shortcut/notes.txt"} {
		p := mustPath(t, name)
		if file, _, err := f.svc.OpenShared(ctx, testUser, root, p); err == nil {
			file.Close()
			t.Fatalf("opened %s outside shared root", name)
		}
		if _, err := f.svc.StatShared(ctx, testUser, root, p); err == nil {
			t.Fatalf("stat %s outside shared root", name)
		}
		var archive bytes.Buffer
		if err := f.svc.WriteSharedArchive(ctx, testUser, root, []Path{p}, &archive); err == nil {
			t.Fatalf("archived %s outside shared root", name)
		}
	}
}
