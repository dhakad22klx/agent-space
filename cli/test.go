package cli

import (
	"fmt"
	"time"
)

func PrintLoop(m *model) {
	for i := 0; i < 10; i++ {
		time.Sleep(3* time.Second)
		m.appendLine(echoStyle.Render(fmt.Sprintf("%d", i)))
	}
}
