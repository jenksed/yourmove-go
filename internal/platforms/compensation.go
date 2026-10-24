package platforms

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

var payAmount = regexp.MustCompile(`(?i)(?:[$£€]|\b(?:USD|CAD|AUD|GBP|EUR)\s*\$?)[ \t]*[0-9]+(?:,[0-9]{3})*(?:\.[0-9]+)?\s*[km]?\b`)
var payContext = regexp.MustCompile(`(?i)\b(?:salary|salaries|base pay|base compensation|compensation|pay range|pay rate|hourly pay|hourly rate|annual pay|hiring range|pays|paying|wage|wages)\b`)
var payPeriod = regexp.MustCompile(`(?i)(?:\b(?:per|an?|each)\s+(?:hour|year|annum|month|week)\b|/\s*(?:hr|hour|year|yr|month|week)\b|\b(?:annually|yearly|hourly)\b)`)
var nonSalary = regexp.MustCompile(`(?i)\b(?:stipend|reimbursement|wellness|insurance|benefits|budget|bonus|equity|stock|revenue|funding|allowance|relocation|tuition|learning|equipment|childcare|child care|pension|retirement|travel|referral|medical|dental|vision|conference registration costs?|annual service costs?)\b`)
var strongPayContext = regexp.MustCompile(`(?i)\b(?:salary|salaries|base pay|pay range|pay rate|hourly rate|hourly pay|annual pay|total compensation|target compensation|wage|wages)\b`)
var payHeading = regexp.MustCompile(`(?i)^(?:base salary(?: range)?|salary(?: range)?|compensation(?: range)?|pay range|hourly(?: pay| rate)?|annual salary|wages?)\s*[:–-]?$`)
var sentenceBreak = regexp.MustCompile(`[.!?][ \t]+`)

// SalaryFromDescription returns an explicit employer excerpt, not an estimated
// range. Currency is mandatory; unlabeled numbers and benefit budgets abstain.
// Keep distinct location/range statements rather than inventing one global pay.
func SalaryFromDescription(description string) string {
	var units []string
	for _, line := range strings.Split(NormalizeDescription(description), "\n") {
		for _, sentence := range sentenceBreak.Split(line, -1) {
			if sentence = strings.TrimSpace(sentence); sentence != "" {
				units = append(units, sentence)
			}
		}
	}
	var excerpts []string
	seen := map[string]bool{}
	for i, line := range units {
		if !payAmount.MatchString(line) {
			continue
		}
		context := payContext.MatchString(line)
		previous := ""
		if !context && i > 0 && !payAmount.MatchString(units[i-1]) && payHeading.MatchString(units[i-1]) {
			previous = units[i-1]
			context = true
		}
		if !context && !payPeriod.MatchString(line) {
			continue
		}
		// A strong salary label elsewhere in a sentence does not make every
		// amount in that sentence base pay (for example, "Salary includes a
		// $1,000 equipment allowance" or salary plus a signing bonus). Abstain
		// when benefit language and a candidate amount share the excerpt.
		if nonSalary.MatchString(line) || (i > 0 && nonSalary.MatchString(units[i-1]) && !strongPayContext.MatchString(line)) {
			continue
		}
		excerpt := line
		if previous != "" {
			excerpt = previous + "\n" + excerpt
		}
		if !payPeriod.MatchString(line) && i+1 < len(units) && len(units[i+1]) <= 60 && payPeriod.MatchString(units[i+1]) && !payAmount.MatchString(units[i+1]) {
			excerpt += "\n" + units[i+1]
		}
		if len(excerpt) > 500 {
			continue
		}
		if !seen[excerpt] {
			excerpts = append(excerpts, excerpt)
			seen[excerpt] = true
		}
	}
	result := strings.Join(excerpts, "\n")
	if len(result) > 500 {
		return ""
	} // Do not truncate a range or its qualifications.
	return result
}

func completeMetadata(metadata Metadata) Metadata {
	if strings.TrimSpace(metadata.SalaryText) == "" {
		metadata.SalaryText = SalaryFromDescription(metadata.Description)
		if metadata.SalaryText != "" {
			metadata.SalaryOrigin = "description"
		}
	} else {
		metadata.SalaryOrigin = "employer"
	}
	return metadata
}

// Optional JSON-LD pay must not make an otherwise readable posting fail.
func salaryFromJSONLD(raw json.RawMessage) string {
	var data struct {
		Currency string `json:"currency"`
		Value    struct {
			Min    *float64 `json:"minValue"`
			Max    *float64 `json:"maxValue"`
			Amount *float64 `json:"value"`
			Unit   string   `json:"unitText"`
		} `json:"value"`
	}
	if json.Unmarshal(raw, &data) != nil {
		return ""
	}
	r := leverSalaryRange{Currency: data.Currency, Interval: data.Value.Unit}
	if data.Value.Min != nil && data.Value.Max != nil {
		r.Min = *data.Value.Min
		r.Max = *data.Value.Max
		return r.display()
	}
	if data.Value.Amount != nil {
		r.Min = *data.Value.Amount
		r.Max = r.Min
		if r.display() != "" {
			return r.Currency + " " + strconv.FormatFloat(r.Min, 'f', -1, 64) + " / " + r.Interval
		}
	}
	return ""
}
