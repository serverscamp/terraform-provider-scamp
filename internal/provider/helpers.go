package provider

import "fmt"

func fmtInt(v int64) string { return fmt.Sprintf("%d", v) }

func toInt64Ptr(p *int) int64 {
    if p == nil { return 0 }
    return int64(*p)
}
func toFloat64Ptr(p *float64) float64 {
    if p == nil { return 0 }
    return *p
}
func ptrFloat64(p *float64) *float64 { if p == nil { return nil }; return p }

// toID converts task ResourceID (string|float64) to int64
func toID(v any) (int64, bool) {
    switch t := v.(type) {
    case float64:
        return int64(t), true
    case string:
        if t == "" { return 0, false }
        var n int64
        _, err := fmt.Sscanf(t, "%d", &n)
        return n, err == nil
    default:
        return 0, false
    }
}
