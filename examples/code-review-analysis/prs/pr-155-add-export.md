# PR #155: Add CSV export for reports

**Author:** ivan
**Branch:** feature/csv-export
**Merged:** 2024-01-20
**Reviewers:** frank
**Files changed:** 4
**Additions:** 156, Deletions:** 8

## Description
Adds ability to export reports as CSV files. Users can export from the reports page.

## Review Comments

**frank:** Looks good, merging.

## Files Changed

### reports/export.go (+112, -0)
```go
func (s *ReportService) ExportCSV(reportID string, w io.Writer) error {
    report, err := s.getReport(reportID)
    if err != nil {
        return err
    }

    writer := csv.NewWriter(w)

    // Write headers
    writer.Write(report.Headers)

    // Write rows
    for _, row := range report.Rows {
        writer.Write(row)
    }

    writer.Flush()
    return writer.Error()
}
```

### reports/handler.go (+36, -8)
Added export endpoint.

### web/reports.tsx (+8, -0)
Added export button.

## Tests
None added.

## CI Status
All checks passed.
