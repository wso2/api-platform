package policies

// RequestAction marker interface (oneof pattern)
type RequestAction interface {
	isRequestAction()     // private marker method
	StopExecution() bool  // returns true if execution should stop
}

// ResponseAction marker interface (oneof pattern)
type ResponseAction interface {
	isResponseAction()    // private marker method
	StopExecution() bool  // returns true if execution should stop
}

// UpstreamRequestModifications - continue request to upstream with modifications
type UpstreamRequestModifications struct {
	SetHeaders    map[string]string      // Set or replace headers
	RemoveHeaders []string               // Headers to remove
	AppendHeaders map[string][]string    // Headers to append
	Body          []byte                 // nil = no change, []byte{} = clear
	Path          *string                // nil = no change
	Method        *string                // nil = no change
}

func (u UpstreamRequestModifications) isRequestAction() {}
func (u UpstreamRequestModifications) StopExecution() bool {
	return false // Continue to next policy
}

// ImmediateResponse - short-circuit and return response immediately
type ImmediateResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

func (i ImmediateResponse) isRequestAction() {}
func (i ImmediateResponse) StopExecution() bool {
	return true // Stop chain, return response immediately
}

// UpstreamResponseModifications - modify response from upstream
type UpstreamResponseModifications struct {
	SetHeaders    map[string]string      // Set or replace headers
	RemoveHeaders []string               // Headers to remove
	AppendHeaders map[string][]string    // Headers to append
	Body          []byte                 // nil = no change, []byte{} = clear
	StatusCode    *int                   // nil = no change
}

func (u UpstreamResponseModifications) isResponseAction() {}
func (u UpstreamResponseModifications) StopExecution() bool {
	return false // Continue to next policy
}
