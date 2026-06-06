package cli

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type progressPrinter struct {
	mu         sync.Mutex
	phase      string
	phaseStart time.Time
	lastLen    int
}

func newProgressPrinter() *progressPrinter {
	return &progressPrinter{}
}

func (p *progressPrinter) Update(phase string, percent float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.phase != phase {
		p.phase = phase
		p.phaseStart = time.Now()
	}

	message := ""
	if percent < 0 {
		message = fmt.Sprintf("%s... %s elapsed", phase, time.Since(p.phaseStart).Round(time.Second))
	} else {
		message = fmt.Sprintf("%s: %.1f%%", phase, percent)
	}

	p.print(message)
}

func (p *progressPrinter) Done() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.lastLen == 0 {
		return
	}

	fmt.Printf("\r%s\r", strings.Repeat(" ", p.lastLen))
	p.lastLen = 0
}

func (p *progressPrinter) print(message string) {
	padding := ""
	if p.lastLen > len(message) {
		padding = strings.Repeat(" ", p.lastLen-len(message))
	}

	fmt.Printf("\r%s%s", message, padding)
	p.lastLen = len(message)
}
