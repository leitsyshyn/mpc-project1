package index

import "math/bits"

type sharedTable interface {
	add(word []byte, doc int)
	index() Index
}

func shared(newTable func(texts [][]byte) sharedTable) BuildFunc {
	return func(texts [][]byte, o Options) []Index {
		t := newTable(texts)
		parallel(texts, o, func(_, doc int) {
			for w := range o.words(texts[doc]) {
				t.add(w, doc)
			}
		})
		return []Index{t.index()}
	}
}

func bucketCount(texts [][]byte) int {
	size := 0
	for _, text := range texts {
		size += len(text)
	}
	return 1 << bits.Len(uint(size/64))
}

func bucket(hash uint64, doc, buckets int) int {
	return int((hash + uint64(doc+1)*0x9e3779b97f4a7c15) & uint64(buckets-1))
}
