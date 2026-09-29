// This is not a real Go module. It exists only so that Go tooling (go vet,
// staticcheck, nilaway, ... run as `./...` from the repo root) ignores this
// directory: a directory with its own go.mod is excluded from the parent
// module's package patterns. Without it, .go files that some npm packages ship
// inside node_modules (e.g. flatted/golang) get analyzed as if they were ours.
module github.com/chrisl8/robot_tracker_go/ui

go 1.24.0
