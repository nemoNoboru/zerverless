package deploy

// DeployStore defines the interface for deployment storage (both in-memory and persistent)
type DeployStore interface {
	Set(d *Deployment) error
	Get(user, path string) (*Deployment, error)
	Delete(user, path string) error
	List() ([]*Deployment, error)
}

