package ui

type AddObstacleRequest struct {
	PixelTopLeft     [2]int  `json:"pixel_top_left"`
	PixelBottomRight [2]int  `json:"pixel_bottom_right"`
	Name             string  `json:"name"`
	Clearance        float64 `json:"clearance"`
}

type UpdateObstacleRequest struct {
	PixelTopLeft     [2]int  `json:"pixel_top_left"`
	PixelBottomRight [2]int  `json:"pixel_bottom_right"`
	Name             string  `json:"name"`
	Clearance        float64 `json:"clearance"`
}

type ObstacleResponse struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	PixelTopLeft     [2]int     `json:"pixel_top_left"`
	PixelBottomRight [2]int     `json:"pixel_bottom_right"`
	WorldTopLeft     [2]float64 `json:"world_top_left"`
	WorldBottomRight [2]float64 `json:"world_bottom_right"`
	Clearance        float64    `json:"clearance"`
}

type ObstaclesListResponse struct {
	Obstacles []ObstacleResponse `json:"obstacles"`
	Count     int                `json:"count"`
	Saved     bool               `json:"saved"`
}

type SaveObstaclesResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}
