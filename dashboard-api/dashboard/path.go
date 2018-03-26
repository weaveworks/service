package dashboard

import "fmt"

// Path encodes the position of a Dashboard element.
//
// Paths to a section are expressed as {s, -1, -1}
// Paths to a row are expressed as {s, r, -1}
// Paths to a panel are expressed as {s, r, p}
type Path struct {
	section, row, panel int
}

func (p *Path) String() string {
	return fmt.Sprintf("%d:%d:%d", p.section, p.row, p.panel)
}
