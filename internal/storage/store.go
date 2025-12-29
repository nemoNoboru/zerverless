package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Store struct {
	baseDir string
}

func NewStore(baseDir string) (*Store, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("create base dir: %w", err)
	}
	return &Store{baseDir: baseDir}, nil
}

func (s *Store) namespaceDir(namespace string) string {
	return filepath.Join(s.baseDir, namespace)
}

func (s *Store) filePath(namespace, path string) (string, error) {
	// Prevent path traversal
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("invalid path: %s", path)
	}
	
	nsDir := s.namespaceDir(namespace)
	fullPath := filepath.Join(nsDir, path)
	
	// Ensure path is within namespace directory
	if !strings.HasPrefix(fullPath, nsDir) {
		return "", fmt.Errorf("path traversal detected: %s", path)
	}
	
	return fullPath, nil
}

func (s *Store) Put(namespace, path string, content []byte) error {
	fullPath, err := s.filePath(namespace, path)
	if err != nil {
		return err
	}

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	return os.WriteFile(fullPath, content, 0644)
}

func (s *Store) Get(namespace, path string) ([]byte, error) {
	fullPath, err := s.filePath(namespace, path)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("read file: %w", err)
	}

	return content, nil
}

func (s *Store) Delete(namespace, path string) error {
	fullPath, err := s.filePath(namespace, path)
	if err != nil {
		return err
	}

	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", path)
		}
		return fmt.Errorf("delete file: %w", err)
	}

	return nil
}

func (s *Store) List(namespace, prefix string) ([]string, error) {
	nsDir := s.namespaceDir(namespace)
	
	var files []string
	err := filepath.Walk(nsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if info.IsDir() {
			return nil
		}
		
		// Get relative path from namespace directory
		relPath, err := filepath.Rel(nsDir, path)
		if err != nil {
			return err
		}
		
		// Filter by prefix
		if prefix == "" || strings.HasPrefix(relPath, prefix) {
			files = append(files, relPath)
		}
		
		return nil
	})
	
	return files, err
}

func (s *Store) Exists(namespace, path string) bool {
	fullPath, err := s.filePath(namespace, path)
	if err != nil {
		return false
	}
	_, err = os.Stat(fullPath)
	return err == nil
}


