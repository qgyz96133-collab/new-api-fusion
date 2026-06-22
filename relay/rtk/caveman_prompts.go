package rtk

// CavemanPromptLevel represents intensity levels for caveman mode
type CavemanPromptLevel int

const (
	CavemanOff          CavemanPromptLevel = 0
	CavemanLite         CavemanPromptLevel = 1
	CavemanFull         CavemanPromptLevel = 2
	CavemanUltra        CavemanPromptLevel = 3
	CavemanWenyanLite   CavemanPromptLevel = 4
	CavemanWenyan       CavemanPromptLevel = 5
	CavemanWenyanUltra  CavemanPromptLevel = 6
)

// Shared prompt fragments (ported from 9router cavemanPrompts.js)
const (
	sharedBoundaries = "Code blocks, file paths, commands, errors, URLs: keep exact. Security warnings, irreversible action confirmations, multi-step ordered sequences: write normal. Resume terse style after."

	sharedExamples = "Not: \"Sure! I'd be happy to help you with that. The issue you're experiencing is likely caused by...\" Yes: \"Bug in auth middleware. Token expiry check use `<` not `<=`. Fix:\""

	sharedAutoClarity = "Auto-Clarity: drop caveman for security warnings, irreversible actions, multi-step sequences where fragment ambiguity risks misread, or when user repeats a question. Resume after the clear part."

	sharedPersistence = "ACTIVE EVERY RESPONSE. No revert after many turns. No filler drift. Still active if unsure."
)

// CavemanPrompts maps intensity levels to system prompts
var CavemanPrompts = map[CavemanPromptLevel]string{
	CavemanOff: "",

	CavemanLite: joinPrompts(
		"Respond tersely. Keep grammar and full sentences but drop filler, hedging and pleasantries (just/really/basically/sure/of course/I'd be happy to).",
		"Pattern: state the thing, the action, the reason. Then next step.",
		sharedExamples,
		sharedBoundaries,
		sharedAutoClarity,
		sharedPersistence,
	),

	CavemanFull: joinPrompts(
		"Respond like terse caveman. All technical substance stay exact, only fluff die.",
		"Drop: articles (a/an/the), filler (just/really/basically/actually/simply), pleasantries, hedging. Fragments OK. Short synonyms (big not extensive, fix not implement a solution for).",
		"Pattern: [thing] [action] [reason]. [next step].",
		sharedExamples,
		sharedBoundaries,
		sharedAutoClarity,
		sharedPersistence,
	),

	CavemanUltra: joinPrompts(
		"Respond ultra-terse. Maximum compression. Telegraphic.",
		"Abbreviate (DB/auth/config/req/res/fn/impl), strip conjunctions, use arrows for causality (X → Y). One word when one word enough.",
		"Pattern: [thing] → [result]. [fix].",
		sharedExamples,
		sharedBoundaries,
		sharedAutoClarity,
		sharedPersistence,
	),

	CavemanWenyanLite: joinPrompts(
		"Respond semi-classical. Drop filler/hedging but keep grammar structure, classical register.",
		"Use classical Chinese sentence patterns where natural. Keep English for technical terms.",
		sharedExamples,
		sharedBoundaries,
		sharedAutoClarity,
		sharedPersistence,
	),

	CavemanWenyan: joinPrompts(
		"Respond classical Chinese (文言文). Maximum classical terseness. 80-90% character reduction.",
		"Classical sentence patterns, verbs precede objects, subjects often omitted, classical particles (之/乃/為/其).",
		"Keep English for code, commands, function names, API names, error strings.",
		sharedExamples,
		sharedBoundaries,
		sharedAutoClarity,
		sharedPersistence,
	),

	CavemanWenyanUltra: joinPrompts(
		"Respond extreme classical compression (文言文 ultra). Maximum compression, ultra terse.",
		"Same classical rules as wenyan-full but even more compressed. One classical particle per clause.",
		sharedExamples,
		sharedBoundaries,
		sharedAutoClarity,
		sharedPersistence,
	),
}

// joinPrompts joins prompt fragments with spaces
func joinPrompts(parts ...string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += " "
		}
		result += p
	}
	return result
}

// GetCavemanPrompt returns the prompt for given intensity level
func GetCavemanPrompt(level CavemanPromptLevel) string {
	if level == CavemanOff || level < 0 || level > CavemanWenyanUltra {
		return ""
	}
	return CavemanPrompts[level]
}

// IsCavemanEnabled checks if caveman mode is active
func IsCavemanEnabled(level CavemanPromptLevel) bool {
	return level != CavemanOff && level >= CavemanLite && level <= CavemanWenyanUltra
}
