package cli

import (
	"fmt"
)

func PrintLoop(m *model) {
	for i := 0; i < 10; i++ {
		m.appendLine(echoStyle.Render(fmt.Sprintf("%d", i)))
	}
}
