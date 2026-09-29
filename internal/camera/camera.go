package camera

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

type Camera interface {
	Start() error
	Stop()
	GetFrame() (*Frame, error)
	GetName() string
}

type CameraConfig struct {
	Type     string `yaml:"type"`
	Name     string `yaml:"name"`
	CameraID int    `yaml:"camera_id"`
	URL      string `yaml:"url"`
	Width    int    `yaml:"width"`
	Height   int    `yaml:"height"`
	FPS      int    `yaml:"fps"`
}

type CameraError struct {
	Message string
}

func (e *CameraError) Error() string {
	return e.Message
}

const DefaultWidth = 640
const DefaultHeight = 480
const DefaultFPS = 30
