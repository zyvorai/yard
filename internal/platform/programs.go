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
	{5, "Intelligent Incident Management", "have"},
	{6, "Safe Automation and Playbook Engine", "have"},
	{7, "Asset Digital Twin and Operations Graph", "have"},
	{8, "Integration Hub", "have"},
	{9, "Yard Intelligence", "have"},
	{10, "Enterprise Identity and Governance", "have"},
	{11, "High Availability and Large-Scale Deployment", "have"},
	{12, "Advanced Field Workforce", "have"},
	{13, "Energy, Sustainability and Cost Intelligence", "have"},
	{14, "Vertical Solution Packs", "have"},
	{15, "Developer and Community Ecosystem", "have"},
	{16, "Commercial Platform", "have"},
}
