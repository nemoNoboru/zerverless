package job

// JobStore defines the interface for job storage (both in-memory and persistent)
type JobStore interface {
	Add(j *Job) error
	Get(id string) (*Job, error)
	NextPending() *Job
	List(limit, offset int, status string) ([]*Job, int)
	ListPending() ([]*Job, error)
	Update(j *Job) error
	SetStatus(id string, status Status, volunteerID string) error
	Complete(id string, result any) error
	Fail(id string, errMsg string) error
	Stats() (pending, running, completed, failed int)
}

