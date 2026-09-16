package renderer

import (
	"strings"
	"testing"
)

func TestFormatMarkdownTables_RAMAndDisk(t *testing.T) {
	input := `Tentu, ini ringkasan penggunaan RAM dan penyimpanan sistem saat ini:

1. Penggunaan RAM (Memory)
| Tipe | Total | Digunakan (Used) | Bebas (Free) | Tersedia (Available) |
| :--- | :--- | :--- | :--- | :--- |
| RAM | 1.9 GiB | 910 MiB | 506 MiB | 1.0 GiB |
| Swap | 4.0 GiB | 17 MiB | 4.0 GiB | - |

> Penggunaan RAM saat ini berada di kisaran ~47%, dengan sekitar 1.0 GiB memori yang masih dapat dialokasikan.

---

2. Penggunaan Storage (Disk)
| Filesystem | Titik Mount | Kapasitas (Size) | Digunakan (Used) | Tersisa (Avail) | Persentase |
| :--- | :--- | :--- | :--- | :--- | :--- |
| /dev/vda1 | / (Root) | 40 GB | 11 GB | 30 GB | 27% |

> Partisi utama sistem baru terpakai 27%, sehingga masih ada sisa ruang penyimpanan sebesar 30 GB.`

	output := FormatMarkdownForTelegram(input)
	t.Logf("\nRendered Output:\n%s\n", output)

	// Verify both tables are wrapped in <pre> blocks
	if strings.Count(output, "<pre>") < 2 {
		t.Errorf("Expected at least 2 <pre> table blocks, got: %s", output)
	}

	// Verify Unicode box drawing characters exist
	if !strings.Contains(output, "┌") || !strings.Contains(output, "┼") || !strings.Contains(output, "└") {
		t.Errorf("Expected Unicode box drawing characters in output, got: %s", output)
	}

	// Verify table contents are preserved
	if !strings.Contains(output, "RAM") || !strings.Contains(output, "1.9 GiB") || !strings.Contains(output, "/dev/vda1") {
		t.Errorf("Table content missing in output: %s", output)
	}

	// Verify blockquotes are formatted
	if !strings.Contains(output, "<blockquote>") {
		t.Errorf("Expected <blockquote> in output, got: %s", output)
	}
}

func TestFormatMarkdownTables_Alignment(t *testing.T) {
	input := `| Item | Qty | Price |
| :--- | :---: | ---: |
| Apple | 10 | $1.50 |
| Orange | 5 | $2.00 |`

	formatted := FormatMarkdownTables(input)
	if !strings.Contains(formatted, "<pre>") {
		t.Fatalf("Table should be wrapped in <pre>")
	}
	if !strings.Contains(formatted, "Apple") || !strings.Contains(formatted, "$1.50") {
		t.Errorf("Table content missing: %s", formatted)
	}
}
