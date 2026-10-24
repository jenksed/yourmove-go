package app

func coverLetterNotice(code string) string {
	return map[string]string{
		"transferring":             "Cover-letter transfer authorized. Review its result below.",
		"generated":                "Cover letter generated and saved locally. Review the text before using it.",
		"saved":                    "Your edits were saved locally. Review the current scope before continuing.",
		"source-description":       "No job description is saved. Open Overview → Edit job and tracking, save the description, then generate again.",
		"source-master-missing":    "No master resume is selected. Open Documents → Manage document on your master resume → Set as master resume, then return and generate again. A filename containing Master does not select it automatically.",
		"source-master-unreadable": "The designated master resume is archived, missing, changed, or has no readable text. Open Documents to repair it or designate a readable DOCX/PDF as master. Changing Resume to emphasize alone will not fix this source.",
		"source-resume-missing":    "Choose an available application resume under Resume to emphasize, then generate again.",
		"source-resume-invalid":    "The writing resume is archived or is the master resume. Choose an active application resume under Resume to emphasize; the master is used separately as career evidence.",
		"source-resume-unreadable": "The resume selected under Resume to emphasize could not be read. Choose another readable DOCX/PDF there, or repair that document in Documents. Your application resume selection is unchanged.",
		"sources":                  "The selected letter source could not be read. Check the job description and master resume, or choose a different DOCX/PDF resume below.",
		"model":                    "Choose a provider and enter a valid model name. For custom providers, set YOURMOVE_COMPAT_URL and YOURMOVE_COMPAT_MODEL.",
		"key":                      "This API provider needs a key. Set the appropriate environment variable, then restart YourMove. API usage is billed separately from subscriptions.",
		"cli":                      "The subscription CLI is not installed or not in YourMove's PATH. Open AI connections for setup instructions.",
		"login":                    "The CLI is not signed in with a subscription account. Open AI connections, then sign in using the official CLI.",
		"cli-failed":               "The subscription CLI could not finish writing. Try it from Terminal, verify your subscription and model availability, and retry.",
		"auth":                     "The provider rejected its API key or permissions. Check the key and account access.",
		"rate":                     "The provider is rate-limiting requests. Try again later or choose another provider.",
		"credits":                  "The provider reports insufficient credits or billing access. Check the provider account.",
		"timeout":                  "Generation timed out. Try a smaller/faster model or a different provider.",
		"output":                   "The model returned no usable letter. Try another model or generate again; existing drafts are unchanged.",
		"provider-unavailable":     "The provider could not complete the request. Check service status and model availability.",
		"model-missing":            "That Ollama model is not installed. Run ollama list, then select an installed model.",
		"offline":                  "Cannot reach Ollama at 127.0.0.1:11434. Start Ollama and try again.",
		"failed":                   "YourMove could not generate or save a cover letter. The existing draft is unchanged. Check the application log.",
		"invalid":                  "The draft could not be saved. Check its length and try again.",
	}[code]
}
