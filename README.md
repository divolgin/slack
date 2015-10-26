# slack
Show task progress in a Slack message

# Example

```go
package main

import (
	"fmt"
	"time"

	"github.com/divolgin/slack"
)

func main() {
	p := &slack.SlackProgress{
		UserToken:    "<slack token>",
		SlackChannel: "#channel-name", // or "@username"
		StatusPrefix: "dev",
	}

	p.Start() // this will start a progress spinner in the specified channel

	go func() {
		// Change progress text every 5 seconds
		p.StatusString = "message 1... "
		time.Sleep(5 * time.Second)
		p.StatusString = "message 2... "
		time.Sleep(5 * time.Second)
		p.StatusString = "message 3... "
		time.Sleep(5 * time.Second)
		p.StatusString = "message 4... "
	}()

	select {
	case err := <-p.ErrorChan:
		fmt.Printf("Failed: %v\n", err)
	case <-time.After(20 * time.Second):
		fmt.Printf("Done\n")
	}

	// Terminate the progress routine
	close(p.StopChan)

	// Allow the routine to clean up the message in Slack
	time.Sleep(5 * time.Second)
}
```
