#!/usr/bin/env python3
from pathlib import Path


def replace_once(path: Path, old: str, new: str) -> None:
    text = path.read_text(encoding="utf-8")
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: expected one replacement site, found {count}")
    path.write_text(text.replace(old, new, 1), encoding="utf-8")


validator = Path("libcontainer/configs/validate/validator.go")
replace_once(
    validator,
    '''func security(config *configs.Config) error {
	// restrict sys without mount namespace
''',
    '''func security(config *configs.Config) error {
	if config.Readonlyfs && !config.Namespaces.Contains(configs.NEWNS) {
		return errors.New("unable to make rootfs read-only without a MNT namespace")
	}

	// restrict sys without mount namespace
''',
)

tests = Path("libcontainer/configs/validate/validator_test.go")
replace_once(
    tests,
    '''func TestValidateSecurityWithoutNEWNS(t *testing.T) {
	config := &configs.Config{
		Rootfs:        "/var",
		MaskPaths:     []string{"/proc/kcore"},
		ReadonlyPaths: []string{"/proc/sys"},
	}

	err := Validate(config)
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

''',
    '''func TestValidateSecurityWithoutNEWNS(t *testing.T) {
	config := &configs.Config{
		Rootfs:        "/var",
		MaskPaths:     []string{"/proc/kcore"},
		ReadonlyPaths: []string{"/proc/sys"},
	}

	err := Validate(config)
	if err == nil {
		t.Error("Expected error to occur but it was nil")
	}
}

func TestValidateReadonlyfsWithoutNEWNS(t *testing.T) {
	config := &configs.Config{
		Rootfs:     "/var",
		Readonlyfs: true,
	}

	err := Validate(config)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	const expected = "unable to make rootfs read-only without a MNT namespace"
	if err.Error() != expected {
		t.Fatalf("unexpected validation error: got %q, want %q", err, expected)
	}
}

func TestValidateReadonlyfsWithNEWNS(t *testing.T) {
	config := &configs.Config{
		Rootfs:     "/var",
		Readonlyfs: true,
		Namespaces: configs.Namespaces{
			{Type: configs.NEWNS},
		},
	}

	if err := Validate(config); err != nil {
		t.Fatalf("expected validation to succeed, got %v", err)
	}
}

''',
)

print("readonlyfs-mountns-candidate-applied")
