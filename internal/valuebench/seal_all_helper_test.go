package valuebench

import valuepkg "github.com/jacoelho/xsd/internal/value"

func sealAll(b *valuepkg.Builder) (*valuepkg.Program, error) {
	keep := make([]bool, b.UserTypeCount())
	for i := range keep {
		keep[i] = true
	}
	return b.SealKeeping(keep)
}
