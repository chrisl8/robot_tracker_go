// Package followsim is a randomized-plant simulator for path followers.
//
// It is deliberately not a model of the Vorpal hexapod. The robot's speed,
// its ability to turn, and how much it slips all change constantly (battery
// level, carpet or hard floor, the feet fitted), and its motion is
// stick-and-lurch rather than steady. So each run draws a different, wide-range,
// unreliable plant, and a follower is judged by how it does across the whole
// spread, never by how it does on one tuning. The only truth the follower gets
// is what the camera would tell it: where the robot is and which way it faces,
// with noise, now.
//
// This is test infrastructure: nothing in the running program imports it. See
// docs/pure-pursuit-plan.md.
package followsim
