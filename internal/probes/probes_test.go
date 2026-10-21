package probes

import "testing"

func TestParseW1Slave(t *testing.T) {
	good := "72 01 4b 46 7f ff 0e 10 57 : crc=57 YES\n72 01 4b 46 7f ff 0e 10 57 t=23125\n"
	c, err := parseW1Slave(good)
	if err != nil || c != 23.125 {
		t.Fatalf("got %v, %v", c, err)
	}

	neg := "5e ff 4b 46 7f ff 02 10 6a : crc=6a YES\n5e ff 4b 46 7f ff 02 10 6a t=-10125\n"
	if c, err := parseW1Slave(neg); err != nil || c != -10.125 {
		t.Fatalf("negative: got %v, %v", c, err)
	}

	bad := []string{
		"72 01 4b 46 7f ff 0e 10 00 : crc=00 NO\n72 01 4b 46 7f ff 0e 10 00 t=23125\n",
		"72 01 4b 46 7f ff 0e 10 57 : crc=57 YES\n72 01 4b 46 7f ff 0e 10 57 t=85000\n",
		"72 01 4b 46 7f ff 0e 10 57 : crc=57 YES\n",
		"",
	}
	for _, s := range bad {
		if _, err := parseW1Slave(s); err == nil {
			t.Errorf("expected error for %q", s)
		}
	}
}
