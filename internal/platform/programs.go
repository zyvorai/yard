// Package platform holds forward-looking contracts for Yard phases 4–16.
// Concrete implementations land as those programs ship; these types keep
// docs, CI, and API versioning aligned.
package platform

// Program is a multi-week product track after the first three deliverables.
type Program struct {
	Order  int
	Name   string
	Status string // planned | partial | have
}

// Programs is the sequenced post-foundation roadmap.
var Programs = []Program{
	{4, "Telemetry Data Platform", "have"},
	{5, "Intelligent Incident Management", "planned"},
	{6, "Safe Automation and Playbook Engine", "planned"},
	{7, "Asset Digital Twin and Operations Graph", "planned"},
	{8, "Integration Hub", "planned"},
	{9, "Yard Intelligence", "planned"},
	{10, "Enterprise Identity and Governance", "planned"},
	{11, "High Availability and Large-Scale Deployment", "planned"},
	{12, "Advanced Field Workforce", "planned"},
	{13, "Energy, Sustainability and Cost Intelligence", "planned"},
	{14, "Vertical Solution Packs", "planned"},
	{15, "Developer and Community Ecosystem", "planned"},
	{16, "Commercial Platform", "planned"},
}
