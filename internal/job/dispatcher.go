package job

// AssignFunc is called to send a job to a volunteer. Returns true if successful.
type AssignFunc func(job *Job, volunteerID string) bool

type Dispatcher struct {
	store    JobStore
	assignFn AssignFunc
}

func NewDispatcher(store JobStore, assignFn AssignFunc) *Dispatcher {
	return &Dispatcher{store: store, assignFn: assignFn}
}

// TryDispatch attempts to assign a pending job to the given volunteer.
func (d *Dispatcher) TryDispatch(volunteerID string) {
	j := d.store.NextPending()
	if j == nil {
		return
	}

	// Mark as running before sending
	d.store.SetStatus(j.ID, StatusRunning, volunteerID)

	if !d.assignFn(j, volunteerID) {
		// Assignment failed, revert to pending
		d.store.SetStatus(j.ID, StatusPending, "")
	}
}
