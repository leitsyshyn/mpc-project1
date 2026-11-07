package index

func sequential(texts [][]byte, o Options) []Index {
	ix := Index{}
	for doc, text := range texts {
		for w := range o.words(text) {
			ix.Add(w, doc)
		}
	}
	return []Index{ix}
}
