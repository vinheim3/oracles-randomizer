package main

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"

	"gopkg.in/yaml.v2"
)

// names corresponding to the constant indices from the rom package.
var gameNames = []string{"", "ages", "seasons"}

type hinter struct {
	areas map[string]string
	items map[string]string
}

// returns a new hinter initialized for the given game.
func newHinter(game int) *hinter {
	h := &hinter{
		areas: make(map[string]string),
		items: make(map[string]string),
	}

	// load item names
	itemFiles := []string{
		"/hints/common_items.yaml",
		fmt.Sprintf("/hints/%s_items.yaml", gameNames[game]),
	}
	for _, filename := range itemFiles {
		if err := yaml.Unmarshal(
			FSMustByte(false, filename), h.items); err != nil {
			panic(err)
		}
	}

	// load area names
	rawAreas := make(map[string][]string)
	areasFilename := fmt.Sprintf("/hints/%s_areas.yaml", gameNames[game])
	if err := yaml.Unmarshal(
		FSMustByte(false, areasFilename), rawAreas); err != nil {
		panic(err)
	}

	// transform the areas map from: {final: [internal 1, internal 2]}
	// to: {internal 1: final, internal 2: final}
	for k, a := range rawAreas {
		for _, v := range a {
			h.areas[v] = k
		}
	}

	return h
}

// returns a randomly generated map of owl names to owl messages.
func (h *hinter) generate(src *rand.Rand, g graph, checks map[*node]*node,
	owlNames []string, plan map[string]string) (map[string]string, error) {
	// function body starts here lol
	hints := make(map[string]string)
	slots := getShuffledSlots(src, checks)
	i := 0

	// check for invalid plando owl names
	for k := range plan {
		found := false
		for _, name := range owlNames {
			if k == name {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("unknown owl name: %s", k)
		}
	}

	// keep track of which slots have been hinted at in order to avoid
	// duplicates. in practice the implementation of the hint loop makes this
	// very unlikely in the first place.
	hintedSlots := make(map[*node]bool)

	for _, owlName := range owlNames {
		// use planned hints if given
		if v, ok := plan[owlName]; ok {
			if !isValidGameText(v) {
				return nil, fmt.Errorf("invalid hint text: %s", v)
			}
			hints[owlName] = h.format(strings.Replace(v, `"`, "", 2))
			continue
		}

		// sometimes owls are just unreachable, so anything goes, i guess
		g.clearMarks()
		owlUnreachable := !g[owlName].getMark(false).reachable()

		for {
			slot, item := slots[i], checks[slots[i]]
			i = (i + 1) % len(slots)

			if hintedSlots[slot] {
				continue
			}

			// don't give hints about checks that are required to reach the owl
			// in the first place, as dictated by the logic of the seed.
			item.removeParent(slot)
			g.clearMarks()
			required := !g[owlName].getMark(false).reachable()
			item.addParent(slot)

			if !required || owlUnreachable {
				hints[owlName] = h.format(fmt.Sprintf("%s holds %s.",
					h.areas[slot.name], h.items[item.name]))
				hintedSlots[slot] = true
				break
			}
		}
	}

	return hints, nil
}

// formats a string for a text box. text box. this doesn't include control
// characters, except for newlines.
func (h *hinter) format(s string) string {
	// split message into words to be wrapped
	words := strings.Split(s, " ")

	// build message line by line
	msg := new(strings.Builder)
	line := ""
	for _, word := range words {
		if len(line) == 0 {
			line += word
		} else if len(line)+len(word) <= 15 {
			line += " " + word
		} else {
			msg.WriteString(line + "\n")
			line = word
		}
	}
	msg.WriteString(line)

	return msg.String()
}

// implement sort.Interface for []*node
type nodeSlice []*node

func (ns nodeSlice) Len() int {
	return len(ns)
}

func (ns nodeSlice) Less(i, j int) bool {
	return ns[i].name < ns[j].name
}

func (ns nodeSlice) Swap(i, j int) {
	ns[i], ns[j] = ns[j], ns[i]
}

// getShuffledSlots returns a randomly ordered slice of slot nodes.
func getShuffledSlots(src *rand.Rand,
	checks map[*node]*node) []*node {
	// make slice of check names
	slots := make([]*node, len(checks))
	i := 0
	for slot, item := range checks {
		// don't include dungeon items, since dungeon item hints would be
		// useless ("Level 7 holds a Boss Key")
		if getDungeonName(item.name) != "" {
			continue
		}

		// and don't include these checks, since they're dummy slots that
		// aren't actually randomized, or seed trees that the player is
		// guaranteed to know about if they're using seeds.
		switch slot.name {
		case "shop, 20 rupees", "shop, 30 rupees",
			"horon village tree", "south lynna tree":
			continue
		}

		slots[i] = slot
		i++
	}
	slots = slots[:i] // trim to the number of actually hintable checks

	// sort the slots before shuffling to get "deterministic" results
	sort.Sort(nodeSlice(slots))
	src.Shuffle(len(slots), func(i, j int) {
		slots[i], slots[j] = slots[j], slots[i]
	})

	return slots
}

// returns truee iff all the characters in s are in the printable range.
func isValidGameText(s string) bool {
	for _, c := range s {
		if c < ' ' || c > 'z' {
			return false
		}
	}
	return true
}
