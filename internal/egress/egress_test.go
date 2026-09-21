package egress

import "testing"

func TestValidateBlocksMetadata(t *testing.T) {
	p := Production(nil)
	if err := p.Validate("http://169.254.169.254/latest/meta-data"); err == nil {
		t.Fatal("metadata must be blocked")
	}
	if err := p.Validate("http://127.0.0.1:9188"); err == nil {
		t.Fatal("loopback must be blocked in production")
	}
	d := Demo()
	if err := d.Validate("http://127.0.0.1:9188"); err != nil {
		t.Fatalf("demo should allow loopback: %v", err)
	}
}

func TestValidateSchemes(t *testing.T) {
	p := Demo()
	if err := p.Validate("ftp://example.com/x"); err == nil {
		t.Fatal("ftp must be rejected")
	}
	if err := p.ValidateOptional("/api/v1/ingest"); err != nil {
		t.Fatal(err)
	}
}
