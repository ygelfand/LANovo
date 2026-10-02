package dhcp

import "testing"

func TestLeasedCarriesEachChangeOnce(t *testing.T) {
	c := &Client{}
	var got []*Lease
	stop := c.Leased.Listen(func(l *Lease) { got = append(got, l) })
	defer stop()

	l := &Lease{}
	c.set(l)
	c.set(l)
	c.set(nil)
	c.set(nil)

	if len(got) != 2 || got[0] != l || got[1] != nil {
		t.Errorf("heard %v, want the lease then nil", got)
	}
}
