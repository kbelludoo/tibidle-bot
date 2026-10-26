package protocol

import "fmt"

// AllKnownHuntIdentifiers catalogues all hunt IDs from hunts.json for route finding
var AllKnownHuntIdentifiers = []string{
	"hunt-26", "hunt-28", "hunt-30", "hunt-32", "hunt-34", "hunt-38",
	"hunt-43", "hunt-45", "hunt-46", "hunt-49", "hunt-50", "hunt-52",
	"hunt-54", "hunt-55", "hunt-56", "hunt-58", "hunt-60", "hunt-62",
	"hunt-63", "hunt-64", "hunt-66", "hunt-68", "hunt-70", "hunt-72",
	"hunt-74", "hunt-75", "hunt-76", "hunt-78", "hunt-80", "hunt-82",
	"hunt-84", "hunt-85", "hunt-86", "hunt-88", "hunt-90", "hunt-92",
	"hunt-94", "hunt-95", "hunt-96", "hunt-98", "hunt-100", "hunt-102",
	"hunt-104", "hunt-105", "hunt-106", "hunt-108", "hunt-110", "hunt-112",
	"hunt-114", "hunt-115", "hunt-116", "hunt-118", "hunt-120", "hunt-122",
	"hunt-124", "hunt-125", "hunt-126", "hunt-128", "hunt-130", "hunt-132",
	"hunt-134", "hunt-135", "hunt-136", "hunt-138", "hunt-140", "hunt-142",
	"hunt-144", "hunt-145", "hunt-146", "hunt-148", "hunt-150", "hunt-152",
	"hunt-154", "hunt-155", "hunt-156", "hunt-158", "hunt-160", "hunt-162",
	"hunt-164", "hunt-165", "hunt-166", "hunt-168", "hunt-170", "hunt-172",
	"hunt-174", "hunt-175", "hunt-176", "hunt-178", "hunt-180", "hunt-182",
	"hunt-184", "hunt-185", "hunt-186", "hunt-188", "hunt-190", "hunt-192",
	"hunt-194", "hunt-195", "hunt-196", "hunt-198", "hunt-200", "hunt-202",
	"hunt-204", "hunt-205", "hunt-206", "hunt-208", "hunt-210", "hunt-212",
	"hunt-214", "hunt-215", "hunt-216", "hunt-218", "hunt-220",
}

// GetHuntRouteID returns formatted string identifier
func GetHuntRouteID(id int) string {
	return fmt.Sprintf("hunt-%d", id)
}
