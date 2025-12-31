package gitops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	return w.CloneWithSubdir(repoURL, branch, "", auth)
}

// CloneWithSubdir clones a repository and optionally configures sparse checkout for a subdirectory
func (w *Watcher) CloneWithSubdir(repoURL, branch, subdir string, auth *Auth) (string, error) {
	repoPath := w.repoPath(repoURL)
	
	// If already exists, return existing path
	if _, err := os.Stat(repoPath); err == nil {
		// If subdir is specified, return path to subdirectory
		if subdir != "" {
			return filepath.Join(repoPath, subdir), nil
		}
		return repoPath, nil
	}
	
	cloneOptions := &git.CloneOptions{
		URL:           repoURL,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		SingleBranch:  true,
		Progress:      os.Stdout,
		Depth:         1, // Shallow clone for faster performance
	}
	
	// Set up authentication
	if err := w.setAuth(cloneOptions, auth); err != nil {
		return "", fmt.Errorf("set auth: %w", err)
	}
	
	_, err := git.PlainClone(repoPath, false, cloneOptions)
	if err != nil {
		return "", fmt.Errorf("clone: %w", err)
	}
	
	// Configure sparse checkout if subdir is specified
	if subdir != "" {
		if err := w.configureSparseCheckout(repoPath, subdir); err != nil {
			// If sparse checkout fails, clean up and return error
			os.RemoveAll(repoPath)
			return "", fmt.Errorf("configure sparse checkout: %w", err)
		}
		return filepath.Join(repoPath, subdir), nil
	}
	
	return repoPath, nil
}

// configureSparseCheckout sets up sparse checkout for a specific subdirectory
func (w *Watcher) configureSparseCheckout(repoPath, subdir string) error {
	// Get worktree
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("open repo: %w", err)
	}
	
	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("get worktree: %w", err)
	}
	
	// Enable sparse checkout by creating .git/info/sparse-checkout file
	sparseCheckoutPath := filepath.Join(repoPath, ".git", "info", "sparse-checkout")
	sparseCheckoutDir := filepath.Dir(sparseCheckoutPath)
	
	// Ensure directory exists
	if err := os.MkdirAll(sparseCheckoutDir, 0755); err != nil {
		return fmt.Errorf("create sparse-checkout dir: %w", err)
	}
	
	// Write sparse-checkout file with the subdirectory pattern
	// Normalize subdir: remove leading/trailing slashes and ensure it starts with /
	normalizedSubdir := subdir
	if normalizedSubdir != "" {
		normalizedSubdir = filepath.Clean(normalizedSubdir)
		if !filepath.IsAbs(normalizedSubdir) {
			normalizedSubdir = "/" + normalizedSubdir
		}
		// Ensure it ends with /* to include all files in the directory
		if !strings.HasSuffix(normalizedSubdir, "/*") {
			normalizedSubdir = normalizedSubdir + "/*"
		}
	}
	
	content := normalizedSubdir + "\n"
	if err := os.WriteFile(sparseCheckoutPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write sparse-checkout file: %w", err)
	}
	
	// Enable sparse checkout by setting git config (using git command or config file)
	configPath := filepath.Join(repoPath, ".git", "config")
	configContent, err := os.ReadFile(configPath)
	if err == nil {
		// Check if sparse checkout is already enabled
		if !strings.Contains(string(configContent), "sparseCheckout = true") {
			// Append sparse checkout config
			configContent = append(configContent, []byte("\n[core]\n\tsparseCheckout = true\n")...)
			if err := os.WriteFile(configPath, configContent, 0644); err != nil {
				return fmt.Errorf("write git config: %w", err)
			}
		}
	}
	
	// Apply sparse checkout by checking out again
	checkoutOpts := &git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("main"), // Will be updated by caller if needed
		Force:  true,
	}
	
	// Try to get current branch
	head, err := repo.Head()
	if err == nil {
		checkoutOpts.Branch = head.Name()
	}
	
	err = worktree.Checkout(checkoutOpts)
	if err != nil {
		return fmt.Errorf("checkout with sparse: %w", err)
	}
	
	return nil
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
	return w.SyncWithSubdir(repoURL, branch, "", auth)
}

// SyncWithSubdir ensures the repository is cloned with sparse checkout for a subdirectory
func (w *Watcher) SyncWithSubdir(repoURL, branch, subdir string, auth *Auth) (string, error) {
	repoPath := w.repoPath(repoURL)
	
	// Clone if doesn't exist
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		path, err := w.CloneWithSubdir(repoURL, branch, subdir, auth)
		if err != nil {
			return "", fmt.Errorf("clone: %w", err)
		}
		return path, nil
	}
	
	// If subdir is specified, ensure sparse checkout is configured
	if subdir != "" {
		if err := w.configureSparseCheckout(repoPath, subdir); err != nil {
			return "", fmt.Errorf("configure sparse checkout: %w", err)
		}
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
	
	// If subdir is specified, return path to subdirectory
	if subdir != "" {
		return filepath.Join(repoPath, subdir), nil
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

