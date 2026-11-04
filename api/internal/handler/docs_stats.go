package handler

import (
	"fmt"

	"github.com/pypx/goopy/model"
)

// extractionLogLine formats one extraction's counts as key=value pairs, so a log
// search or awk can total them. "partial" is true when any file had a parse error
// or any module panicked: the docs are then missing symbols.
func extractionLogLine(name, version string, pkg *model.Package) string {
	s := pkg.Stats
	return fmt.Sprintf(
		"docs: extracted pkg=%s version=%s files=%d modules=%d published=%d files_with_errors=%d parse_errors=%d panicked=%d partial=%t ms=%d",
		name, version, s.Files, s.Modules, len(pkg.Modules), s.FilesWithErrors, s.Errors, s.PanickedModules,
		s.FilesWithErrors > 0 || s.PanickedModules > 0, s.Duration.Milliseconds(),
	)
}
