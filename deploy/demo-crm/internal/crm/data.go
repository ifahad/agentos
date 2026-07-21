package crm

// Static demo data, consistent with the legacy-ERP seed
// (deploy/initdb/01-legacy-erp.sql): same eight customers in the same order,
// so ids line up with the ERP's serial primary keys. Red Sea Logistics (id 3)
// has exactly two open tickets — the Phase 3 smoke test depends on it.

// Customer mirrors an ERP customer row.
type Customer struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	City    string `json:"city"`
	Segment string `json:"segment"`
}

// Ticket is a CRM support ticket referencing a customer by id.
type Ticket struct {
	ID         int    `json:"id"`
	CustomerID int    `json:"customer_id"`
	Subject    string `json:"subject"`
	Status     string `json:"status"` // open | closed
	OpenedAt   string `json:"opened_at"`
}

var customers = []Customer{
	{1, "Al-Faisal Trading Co.", "Riyadh", "enterprise"},
	{2, "Najd Industrial Supplies", "Riyadh", "smb"},
	{3, "Red Sea Logistics", "Jeddah", "enterprise"},
	{4, "Hejaz Retail Group", "Jeddah", "smb"},
	{5, "Eastern Petro Services", "Dammam", "enterprise"},
	{6, "Qassim Agri Traders", "Buraidah", "smb"},
	{7, "Ministry of Municipal Works", "Riyadh", "government"},
	{8, "Tabuk Construction Partners", "Tabuk", "smb"},
}

var tickets = []Ticket{
	{1, 1, "Payment portal error on invoice for order 4", "closed", "2026-03-02"},
	{2, 1, "Request volume discount review for H2", "open", "2026-06-10"},
	{3, 3, "Shipment tracking API returns stale locations", "open", "2026-06-18"},
	{4, 3, "Customs clearance documents missing for order 9", "open", "2026-07-01"},
	{5, 3, "Warehouse slot booking double-charged in February", "closed", "2026-02-20"},
	{6, 4, "POS integration timeout during peak hours", "open", "2026-05-12"},
	{7, 5, "Update billing contact for Dammam office", "closed", "2026-04-05"},
	{8, 6, "Seasonal pricing not applied to June order", "closed", "2026-06-03"},
	{9, 7, "Procurement portal access for new staff", "open", "2026-05-28"},
	{10, 8, "Credit note requested for cancelled order 20", "closed", "2026-04-14"},
}
