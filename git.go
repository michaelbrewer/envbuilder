package envbuilder

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/sideband"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

type CloneRepoOptions struct {
	Path    string
	Storage billy.Filesystem

	RepoURL      string
	RepoAuth     transport.AuthMethod
	Progress     sideband.Progress
	Insecure     bool
	SingleBranch bool
	Depth        int
	CABundle     []byte
	ProxyOptions transport.ProxyOptions
}

// CloneRepo will clone the repository at the given URL into the given path.
// If a repository is already initialized at the given path, it will not
// be cloned again.
//
// The bool returned states whether the repository was cloned or not.
func CloneRepo(ctx context.Context, opts CloneRepoOptions) (bool, error) {
	parsed, err := url.Parse(opts.RepoURL)
	if err != nil {
		return false, fmt.Errorf("parse url %q: %w", opts.RepoURL, err)
	}
	err = opts.Storage.MkdirAll(opts.Path, 0755)
	if err != nil {
		return false, fmt.Errorf("mkdir %q: %w", opts.Path, err)
	}
	reference := parsed.Fragment
	if reference == "" && opts.SingleBranch {
		reference = "refs/heads/main"
	}
	parsed.RawFragment = ""
	parsed.Fragment = ""
	fs, err := opts.Storage.Chroot(opts.Path)
	if err != nil {
		return false, fmt.Errorf("chroot %q: %w", opts.Path, err)
	}
	gitDir, err := fs.Chroot(".git")
	if err != nil {
		return false, fmt.Errorf("chroot .git: %w", err)
	}
	gitStorage := filesystem.NewStorage(gitDir, cache.NewObjectLRU(cache.DefaultMaxSize*10))
	fsStorage := filesystem.NewStorage(fs, cache.NewObjectLRU(cache.DefaultMaxSize*10))
	repo, err := git.Open(fsStorage, gitDir)
	if errors.Is(err, git.ErrRepositoryNotExists) {
		err = nil
	}
	if err != nil {
		return false, fmt.Errorf("open %q: %w", opts.RepoURL, err)
	}
	if repo != nil {
		return false, nil
	}

	_, err = git.CloneContext(ctx, gitStorage, fs, &git.CloneOptions{
		URL:             parsed.String(),
		Auth:            opts.RepoAuth,
		Progress:        opts.Progress,
		ReferenceName:   plumbing.ReferenceName(reference),
		InsecureSkipTLS: opts.Insecure,
		Depth:           opts.Depth,
		SingleBranch:    opts.SingleBranch,
		CABundle:        opts.CABundle,
		ProxyOptions:    opts.ProxyOptions,
	})
	if errors.Is(err, git.ErrRepositoryAlreadyExists) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("clone %q: %w", opts.RepoURL, err)
	}
	return true, nil
}
