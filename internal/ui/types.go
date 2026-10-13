package ui

// AddObstacleRequest is the body of a request to add a static obstacle: its pixel rectangle, name and clearance.
type AddObstacleRequest struct {
	PixelTopLeft     [2]int  `json:"pixel_top_left"`
	PixelBottomRight [2]int  `json:"pixel_bottom_right"`
	Name             string  `json:"name"`
	Clearance        float64 `json:"clearance"`
}

// UpdateObstacleRequest is the body of a request to change a static obstacle.
type UpdateObstacleRequest struct {
	PixelTopLeft     [2]int  `json:"pixel_top_left"`
	PixelBottomRight [2]int  `json:"pixel_bottom_right"`
	Name             string  `json:"name"`
	Clearance        float64 `json:"clearance"`
}

// ObstacleResponse is a static obstacle as the API returns it, in pixels and world metres.
type ObstacleResponse struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	PixelTopLeft     [2]int     `json:"pixel_top_left"`
	PixelBottomRight [2]int     `json:"pixel_bottom_right"`
	WorldTopLeft     [2]float64 `json:"world_top_left"`
	WorldBottomRight [2]float64 `json:"world_bottom_right"`
	Clearance        float64    `json:"clearance"`
}

// ObstaclesListResponse is the obstacle list, with whether it has been saved to disk.
type ObstaclesListResponse struct {
	Obstacles []ObstacleResponse `json:"obstacles"`
	Count     int                `json:"count"`
	Saved     bool               `json:"saved"`
}

// SaveObstaclesResponse is the result of saving the obstacles.
type SaveObstaclesResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}
