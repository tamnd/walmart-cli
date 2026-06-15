package walmart

import "strings"

// ids.go resolves a reference to a (kind, id) pair and builds canonical URLs,
// all offline. It backs `walmart ref id` and `walmart ref url`, and the Resolver
// the ant host calls to turn a walmart:// URI into the right command.

// Classify reads a reference (a URL, a path, or a bare id) and reports what it
// points at. Kind is one of product, store, category, or unknown.
func Classify(ref string) Ref {
	in := strings.TrimSpace(ref)
	r := Ref{Input: in, Kind: "unknown"}
	if in == "" {
		return r
	}

	// A bare item id: a run of 6 to 12 digits, the shape of a Walmart usItemId.
	if isItemID(in) {
		r.Kind, r.ID = "product", in
		r.URL = URLFor(r.Kind, r.ID)
		return r
	}

	segs := splitSegs(refPath(in))
	if len(segs) == 0 {
		return r
	}

	switch segs[0] {
	case "ip":
		// /ip/<slug>/<id> or /ip/<id>: the id is the last all-digit segment.
		if id := lastDigits(segs[1:]); id != "" {
			r.Kind, r.ID = "product", id
		}
	case "store":
		// /store/<id> or /store/<slug>/<id>: the store number is the last digits.
		if id := lastDigits(segs[1:]); id != "" {
			r.Kind, r.ID = "store", id
		}
	case "cp":
		// /cp/<slug>/<id> or /cp/<id>: the category id is the last digit segment.
		if id := lastDigits(segs[1:]); id != "" {
			r.Kind, r.ID = "category", id
		}
	case "browse":
		// /browse/<slug>/<id> or /browse/...?cat_id=<id>: a category.
		if id := query(in, "cat_id"); id != "" {
			r.Kind, r.ID = "category", id
		} else if id := lastDigits(segs[1:]); id != "" {
			r.Kind, r.ID = "category", id
		}
	}
	if r.Kind != "unknown" {
		r.URL = URLFor(r.Kind, r.ID)
	}
	return r
}

// URLFor builds the canonical Walmart URL for a (kind, id) pair.
func URLFor(kind, id string) string {
	switch kind {
	case "product":
		return BaseURL + "/ip/" + id
	case "store":
		return BaseURL + "/store/" + id
	case "category":
		return BaseURL + "/cp/" + id
	default:
		return ""
	}
}

// catIDFromURL returns the category id named by a /cp/ or /browse URL or path,
// or "" when the reference is not a category. It powers the category edge a
// product's breadcrumb carries.
func catIDFromURL(u string) string {
	if r := Classify(u); r.Kind == "category" {
		return r.ID
	}
	return ""
}

// productID reduces a reference to its bare product id.
func productID(ref string) string {
	if r := Classify(ref); r.Kind == "product" {
		return r.ID
	}
	return strings.Trim(ref, "/")
}

// storeID reduces a reference to its bare store number.
func storeID(ref string) string {
	if r := Classify(ref); r.Kind == "store" {
		return r.ID
	}
	return strings.Trim(ref, "/")
}

// categoryID reduces a reference to its bare category id, or "" for an empty or
// top-level reference.
func categoryID(ref string) string {
	if r := Classify(ref); r.Kind == "category" {
		return r.ID
	}
	s := strings.Trim(ref, "/")
	if isDigits(s) {
		return s
	}
	return ""
}

// refPath reduces a reference to a site path: a full URL loses scheme and host,
// a bare path is returned trimmed.
func refPath(ref string) string {
	if i := strings.Index(ref, "://"); i >= 0 {
		rest := ref[i+3:]
		if s := strings.IndexByte(rest, '/'); s >= 0 {
			rest = rest[s:]
		} else {
			return "/"
		}
		ref = rest
	}
	if q := strings.IndexByte(ref, '?'); q >= 0 {
		ref = ref[:q]
	}
	if !strings.HasPrefix(ref, "/") {
		ref = "/" + ref
	}
	return ref
}

// query returns the value of a query parameter in a URL, or "" when absent.
func query(ref, key string) string {
	q := strings.IndexByte(ref, '?')
	if q < 0 {
		return ""
	}
	for _, pair := range strings.Split(ref[q+1:], "&") {
		k, v, ok := strings.Cut(pair, "=")
		if ok && k == key {
			return v
		}
	}
	return ""
}

func splitSegs(path string) []string {
	var out []string
	for _, s := range strings.Split(path, "/") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// lastDigits returns the last all-digit segment, the shape of a product id in
// /ip/ and a category id in /cp/.
func lastDigits(segs []string) string {
	id := ""
	for _, s := range segs {
		if isDigits(s) {
			id = s
		}
	}
	return id
}

// isItemID reports whether s is a bare Walmart item number: 6 to 12 digits.
func isItemID(s string) bool {
	if len(s) < 6 || len(s) > 12 {
		return false
	}
	return isDigits(s)
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
