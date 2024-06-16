package main

import (
	"context"

	"github.com/redpanda-data/benthos/v4/public/service"

	// Import all standard Benthos components
	_ "github.com/RuneRoven/benthosSMTP"

	// "github.com/redpanda-data/connect/v4/public/components/all"
	_ "github.com/redpanda-data/benthos/v4/public/bloblang"
	_ "github.com/redpanda-data/benthos/v4/public/components/io"
	_ "github.com/redpanda-data/connect/v4/public/components/mqtt"
)

func main() {
	service.RunCLI(context.Background())
}
