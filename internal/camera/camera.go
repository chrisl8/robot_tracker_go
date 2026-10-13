package camera

// Frame is one captured image: raw pixel bytes (OpenCV order, BGR for colour sources) laid out row by row, with the dimensions needed to interpret them.
type Frame struct {
	Data     []byte
	Width    int
	Height   int
	Channels int
	// Seq numbers the frames a camera produces, starting at 1; it is 0 for a
	// source that does not number them. GetFrame can return the same frame
	// repeatedly, so a consumer that sees the same non-zero Seq again has not
	// been given a new frame.
	Seq uint64
}

// Camera is a source of video frames. GetFrame never blocks: it returns the latest frame, which may be the same one twice (see Frame.Seq).
type Camera interface {
	Start() error
	Stop()
	GetFrame() (*Frame, error)
	GetName() string
}

// CameraConfig is one entry of the YAML "cameras" list. A non-empty URL selects an IP/network stream; otherwise CameraID picks a local device. Zero Width, Height or FPS fall back to the Default constants.
type CameraConfig struct {
	Type     string `yaml:"type"`
	Name     string `yaml:"name"`
	CameraID int    `yaml:"camera_id"`
	URL      string `yaml:"url"`
	Width    int    `yaml:"width"`
	Height   int    `yaml:"height"`
	FPS      int    `yaml:"fps"`
}

// CameraError is a camera failure the caller can show to the user as is (for example "camera not available").
type CameraError struct {
	Message string
}

// Error returns the failure message.
func (e *CameraError) Error() string {
	return e.Message
}

// DefaultWidth is the capture width used when the configuration gives none.
const DefaultWidth = 640

// DefaultHeight is the capture height used when the configuration gives none.
const DefaultHeight = 480

// DefaultFPS is the capture frame rate used when the configuration gives none.
const DefaultFPS = 30
