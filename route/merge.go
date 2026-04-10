package route

// mergeLearnedIntoRegistry adds learned triggers and anti-triggers to matching
// registry components. Learned triggers extend existing Keywords; anti-triggers
// extend AntiTriggers.
func mergeLearnedIntoRegistry(reg *Registry, lt *LearnedTriggers) {
	if lt == nil {
		return
	}
	for i := range reg.Components {
		comp := &reg.Components[i]
		keys := componentKeys(comp)
		comp.Keywords = mergeField(comp.Keywords, keys, lt.Triggers)
		comp.AntiTriggers = mergeField(comp.AntiTriggers, keys, lt.AntiTriggers)
	}
}

// componentKeys returns the lookup keys for a component: its Name and Extension (if different).
func componentKeys(comp *Component) []string {
	if comp.Extension != "" && comp.Extension != comp.Name {
		return []string{comp.Name, comp.Extension}
	}
	return []string{comp.Name}
}

// mergeField appends any learned entries matching keys, deduplicating the result.
func mergeField(existing []string, keys []string, learned map[string][]string) []string {
	var added []string
	for _, k := range keys {
		if vals, ok := learned[k]; ok {
			added = append(added, vals...)
		}
	}
	if len(added) == 0 {
		return existing
	}
	return dedupStrings(append(existing, added...))
}
