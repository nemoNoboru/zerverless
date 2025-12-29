package gitops

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

type Auth struct {
	Token    string // Personal access token
	SSHKey   []byte // SSH private key
	Username string // For token auth
}

type Watcher struct {
	baseDir     string
	pollInterval time.Duration
}

func NewWatcher(baseDir string, pollInterval time.Duration) *Watcher {
	return &Watcher{
		baseDir:      baseDir,
		pollInterval: pollInterval,
	}
}

func (w *Watcher) repoPath(repoURL string) string {
	// Use repo URL as directory name (sanitized)
	dirName := filepath.Base(repoURL)
	if dirName == "" || dirName == "." {
		dirName = "repo"
	}
	return filepath.Join(w.baseDir, dirName)
}

func (w *Watcher) Clone(repoURL, branch string, auth *Auth) (string, error) {
	repoPath := w.repoPath(repoURL)
	
	// If already exists, return existing path
	if _, err := os.Stat(repoPath); err == nil {
		return repoPath, nil
	}
	
	cloneOptions := &git.CloneOptions{
		URL:           repoURL,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		SingleBranch:  true,
		Progress:      os.Stdout,
	}
	
	// Set up authentication
	if err := w.setAuth(cloneOptions, auth); err != nil {
		return "", fmt.Errorf("set auth: %w", err)
	}
	
	_, err := git.PlainClone(repoPath, false, cloneOptions)
	if err != nil {
		return "", fmt.Errorf("clone: %w", err)
	}
	
	return repoPath, nil
}

func (w *Watcher) Pull(repoPath, branch string, auth *Auth) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("open repo: %w", err)
	}
	
	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("get worktree: %w", err)
	}
	
	pullOptions := &git.PullOptions{
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		SingleBranch:  true,
	}
	
	if err := w.setAuth(pullOptions, auth); err != nil {
		return fmt.Errorf("set auth: %w", err)
	}
	
	err = worktree.Pull(pullOptions)
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("pull: %w", err)
	}
	
	return nil
}

func (w *Watcher) GetLatestCommit(repoPath, branch string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("open repo: %w", err)
	}
	
	ref, err := repo.Reference(plumbing.NewBranchReferenceName(branch), true)
	if err != nil {
		return "", fmt.Errorf("get reference: %w", err)
	}
	
	return ref.Hash().String(), nil
}

func (w *Watcher) CheckoutBranch(repoPath, branch string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("open repo: %w", err)
	}
	
	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("get worktree: %w", err)
	}
	
	checkoutOptions := &git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branch),
		Force:  false,
	}
	
	err = worktree.Checkout(checkoutOptions)
	if err != nil {
		return fmt.Errorf("checkout: %w", err)
	}
	
	return nil
}

// HasChanges checks if there are new commits in the remote repository
func (w *Watcher) HasChanges(repoPath, branch string, auth *Auth) (bool, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return false, fmt.Errorf("open repo: %w", err)
	}
	
	// Get current local commit
	localRef, err := repo.Reference(plumbing.NewBranchReferenceName(branch), true)
	if err != nil {
		return false, fmt.Errorf("get local reference: %w", err)
	}
	localCommit := localRef.Hash()
	
	// Fetch latest from remote
	remote, err := repo.Remote("origin")
	if err != nil {
		return false, fmt.Errorf("get remote: %w", err)
	}
	
	fetchOptions := &git.FetchOptions{
		RefSpecs: []config.RefSpec{"refs/heads/*:refs/remotes/origin/*"},
	}
	
	if err := w.setAuth(fetchOptions, auth); err != nil {
		return false, fmt.Errorf("set auth: %w", err)
	}
	
	err = remote.Fetch(fetchOptions)
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return false, fmt.Errorf("fetch: %w", err)
	}
	
	// Get remote commit
	remoteRef, err := repo.Reference(plumbing.NewRemoteReferenceName("origin", branch), true)
	if err != nil {
		return false, fmt.Errorf("get remote reference: %w", err)
	}
	remoteCommit := remoteRef.Hash()
	
	// Compare commits
	return localCommit != remoteCommit, nil
}

// Sync ensures the repository is cloned and up to date with the specified branch
func (w *Watcher) Sync(repoURL, branch string, auth *Auth) (string, error) {
	repoPath := w.repoPath(repoURL)
	
	// Clone if doesn't exist
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		_, err := w.Clone(repoURL, branch, auth)
		if err != nil {
			return "", fmt.Errorf("clone: %w", err)
		}
		return repoPath, nil
	}
	
	// Pull latest changes
	err := w.Pull(repoPath, branch, auth)
	if err != nil {
		return "", fmt.Errorf("pull: %w", err)
	}
	
	// Ensure correct branch is checked out
	err = w.CheckoutBranch(repoPath, branch)
	if err != nil {
		return "", fmt.Errorf("checkout: %w", err)
	}
	
	return repoPath, nil
}

func (w *Watcher) setAuth(opts interface{}, auth *Auth) error {
	if auth == nil {
		return nil
	}
	
	// Token authentication
	if auth.Token != "" {
	switch o := opts.(type) {
	case *git.CloneOptions:
		o.Auth = &http.BasicAuth{
			Username: auth.Username,
			Password: auth.Token,
		}
	case *git.PullOptions:
		o.Auth = &http.BasicAuth{
			Username: auth.Username,
			Password: auth.Token,
		}
	case *git.FetchOptions:
		o.Auth = &http.BasicAuth{
			Username: auth.Username,
			Password: auth.Token,
		}
	}
	return nil
	}
	
	// SSH authentication
	if len(auth.SSHKey) > 0 {
		sshAuth, err := ssh.NewPublicKeys("git", auth.SSHKey, "")
		if err != nil {
			return fmt.Errorf("create ssh auth: %w", err)
		}
		
		switch o := opts.(type) {
		case *git.CloneOptions:
			o.Auth = sshAuth
		case *git.PullOptions:
			o.Auth = sshAuth
		case *git.FetchOptions:
			o.Auth = sshAuth
		}
		return nil
	}
	
	return nil
}

