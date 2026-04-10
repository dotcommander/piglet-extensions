package safeguard

func defaultPatterns() []string {
	return []string{
		`\brm\s+-(r|f|rf|fr)\b`,
		`\brm\s+-\w*(r|f)\w*\s+/`,
		`\bsudo\s+rm\b`,
		`\bmkfs\b`,
		`\bdd\s+if=`,
		`\b(DROP|TRUNCATE)\s+(TABLE|DATABASE|SCHEMA)\b`,
		`\bDELETE\s+FROM\s+\S+\s*;?\s*$`,
		`\bgit\s+push\s+.*--force\b`,
		`\bgit\s+reset\s+--hard\b`,
		`\bgit\s+clean\s+-[dfx]`,
		`\bgit\s+branch\s+-D\b`,
		`\bchmod\s+-R\s+777\b`,
		`\bchown\s+-R\b`,
		`>\s*/dev/sd[a-z]`,
		`\b:()\s*\{\s*:\|:\s*&\s*\}\s*;?\s*:`,
		`\bkill\s+-9\s+-1\b`,
		`\bshutdown\b`,
		`\breboot\b`,
		`\bsystemctl\s+(stop|disable|mask)\b`,
	}
}
