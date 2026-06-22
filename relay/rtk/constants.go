package rtk

// RTK compression constants ported from 9Router open-sse/rtk/constants.js

const (
	// Max input size to compress (10MB)
	MaxInputSize = 10 * 1024 * 1024

	// MinCompressSize - minimum content length to attempt compression
	MinCompressSize = 500

	// DetectWindow - number of bytes to examine for pattern detection
	DetectWindow = 1024

	// Git diff constants
	GitDiffHunkMaxLines = 100
	GitDiffContextKeep  = 3

	// Git status constants
	StatusMaxFiles     = 10
	StatusMaxUntracked = 10

	// Grep constants
	GrepPerFileMax = 10

	// Find constants
	FindPerDirMax  = 10
	FindTotalDirMax = 20

	// Dedup log constants
	DedupLogMax = 2000

	// LS constants
	LSExtSummaryTop = 5

	// Tree constants
	TreeMaxLines = 200

	// Smart truncate constants
	SmartTruncateHead     = 120
	SmartTruncateTail     = 60
	SmartTruncateMinLines = 250

	// Read numbered constants
	ReadNumberedMinLines   = 250
	ReadNumberedMinHitRatio = 0.7

	// Search list constants
	SearchListPerDirMax  = 10
	SearchListTotalDirMax = 20

	// Build output constants
	BuildOutputDeprecationMax = 3
	BuildOutputWarningMax     = 5
)

// Noise directories to filter out in various filters
var NoiseDirs = []string{
	"node_modules",
	".git",
	"target",
	"__pycache__",
	".next",
	"dist",
	"build",
	".venv",
	"venv",
	".cache",
	".idea",
	".vscode",
	".DS_Store",
}
