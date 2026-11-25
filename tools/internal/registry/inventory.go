package registry

import (
	"errors"
	"fmt"
	"io"
)

const (
	activeStatus  = "active"
	retiredStatus = "retired"
)

// Inventory records published error codes in publication order.
type Inventory struct {
	order   []string
	entries map[string]publishedCode
}

type publishedCode struct {
	class     string
	retryable bool
	retired   bool
}

type rawPublishedCode struct {
	Code      string `yaml:"code"`
	Class     string `yaml:"class"`
	Retryable *bool  `yaml:"retryable"`
	Status    string `yaml:"status"`
}

// LoadInventory reads a published inventory with strict YAML decoding.
func LoadInventory(src io.Reader) (Inventory, error) {
	var raw struct {
		Codes []rawPublishedCode `yaml:"codes"`
	}
	if err := decodeYAML(src, "errors.published.yaml", &raw); err != nil {
		return Inventory{}, err
	}
	if raw.Codes == nil {
		return Inventory{}, errors.New("errors.published.yaml must set a codes list")
	}
	inventory := newInventory(len(raw.Codes))
	for _, entry := range raw.Codes {
		if err := inventory.addPublished(entry); err != nil {
			return Inventory{}, err
		}
	}
	return inventory, nil
}

// InventoryFromRegistry builds an active baseline in registry order.
func InventoryFromRegistry(f File) Inventory {
	inventory := newInventory(len(f.Codes))
	for _, entry := range f.Codes {
		inventory.add(entry.Code, publishedCode{class: entry.Class, retryable: entry.Retryable})
	}
	return inventory
}

func newInventory(size int) Inventory {
	return Inventory{
		order:   make([]string, 0, size),
		entries: make(map[string]publishedCode, size),
	}
}

func (i *Inventory) addPublished(entry rawPublishedCode) error {
	switch {
	case entry.Code == "" || entry.Class == "" || entry.Retryable == nil:
		return fmt.Errorf("inventory code %q must set code, class and retryable", entry.Code)
	case entry.Status != activeStatus && entry.Status != retiredStatus:
		return fmt.Errorf("inventory code %q has invalid status %q", entry.Code, entry.Status)
	}
	if _, dup := i.entries[entry.Code]; dup {
		return fmt.Errorf("inventory code %q is listed twice", entry.Code)
	}
	i.add(entry.Code, publishedCode{
		class: entry.Class, retryable: *entry.Retryable, retired: entry.Status == retiredStatus,
	})
	return nil
}

func (i *Inventory) add(code string, entry publishedCode) {
	i.order = append(i.order, code)
	i.entries[code] = entry
}

// CheckAppendOnly rejects changes to published entries except retirement.
// Existing entries must retain their order, with new codes added at the end.
func CheckAppendOnly(base, current Inventory) error {
	for pos, code := range base.order {
		got, exists := current.entries[code]
		if !exists {
			return fmt.Errorf("inventory code %q was removed", code)
		}
		if current.order[pos] != code {
			return fmt.Errorf(
				"inventory code %q changed position; new codes must be appended",
				code,
			)
		}
		if err := checkPublishedEntry(code, base.entries[code], got); err != nil {
			return err
		}
	}
	return nil
}

func checkPublishedEntry(code string, want, got publishedCode) error {
	switch {
	case got.class != want.class || got.retryable != want.retryable:
		return fmt.Errorf("inventory code %q has different class or retryable values", code)
	case want.retired && !got.retired:
		return fmt.Errorf("retired code %q cannot be reactivated", code)
	default:
		return nil
	}
}

// CheckInventory checks that active entries match the registry and retired
// entries do not appear in it.
func CheckInventory(f File, inventory Inventory) error {
	active := make(map[string]Entry, len(f.Codes))
	for _, entry := range f.Codes {
		if _, ok := inventory.entries[entry.Code]; !ok {
			return fmt.Errorf("registry code %q is missing from the inventory", entry.Code)
		}
		active[entry.Code] = entry
	}
	for _, code := range inventory.order {
		entry, exists := active[code]
		if err := inventory.entries[code].checkRegistry(code, entry, exists); err != nil {
			return err
		}
	}
	return nil
}

func (p publishedCode) checkRegistry(code string, entry Entry, exists bool) error {
	if p.retired {
		if exists {
			return fmt.Errorf("retired code %q is in the registry", code)
		}
		return nil
	}
	if !exists {
		return fmt.Errorf("active code %q is missing from the registry", code)
	}
	if p.class != entry.Class || p.retryable != entry.Retryable {
		return fmt.Errorf("code %q has different class or retryable values", code)
	}
	return nil
}
