package secrets

type acNode struct {
	children map[byte]*acNode
	fail     *acNode
	output   []int
}

type AhoCorasick struct {
	root     *acNode
	patterns []string
}

func NewAhoCorasick(patterns []string) *AhoCorasick {
	ac := &AhoCorasick{
		root:     &acNode{children: make(map[byte]*acNode)},
		patterns: patterns,
	}
	ac.build()
	return ac
}

func (ac *AhoCorasick) build() {
	for i, pat := range ac.patterns {
		node := ac.root
		for j := 0; j < len(pat); j++ {
			c := pat[j]
			if node.children[c] == nil {
				node.children[c] = &acNode{children: make(map[byte]*acNode)}
			}
			node = node.children[c]
		}
		node.output = append(node.output, i)
	}

	queue := make([]*acNode, 0)
	for _, child := range ac.root.children {
		child.fail = ac.root
		queue = append(queue, child)
	}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		for c, child := range curr.children {
			queue = append(queue, child)

			fail := curr.fail
			for fail != nil && fail.children[c] == nil {
				fail = fail.fail
			}
			if fail == nil {
				child.fail = ac.root
			} else {
				child.fail = fail.children[c]
			}
			if child.fail == child {
				child.fail = ac.root
			}
			child.output = append(child.output, child.fail.output...)
		}
	}
}

type ACMatch struct {
	PatternIndex int
	Position     int
}

func (ac *AhoCorasick) Search(text string) []ACMatch {
	var matches []ACMatch
	node := ac.root

	for i := 0; i < len(text); i++ {
		c := text[i]
		for node != ac.root && node.children[c] == nil {
			node = node.fail
		}
		if node.children[c] != nil {
			node = node.children[c]
		}
		for _, idx := range node.output {
			matches = append(matches, ACMatch{
				PatternIndex: idx,
				Position:     i - len(ac.patterns[idx]) + 1,
			})
		}
	}

	return matches
}
