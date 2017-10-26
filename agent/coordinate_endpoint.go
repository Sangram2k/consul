package agent

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
)

// coordinateDisabled handles all the endpoints when coordinates are not enabled,
// returning an error message.
func coordinateDisabled(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	resp.WriteHeader(http.StatusUnauthorized)
	fmt.Fprint(resp, "Coordinate support disabled")
	return nil, nil
}

// sorter wraps a coordinate list and implements the sort.Interface to sort by
// node name.
type sorter struct {
	coordinates structs.Coordinates
}

// See sort.Interface.
func (s *sorter) Len() int {
	return len(s.coordinates)
}

// See sort.Interface.
func (s *sorter) Swap(i, j int) {
	s.coordinates[i], s.coordinates[j] = s.coordinates[j], s.coordinates[i]
}

// See sort.Interface.
func (s *sorter) Less(i, j int) bool {
	return s.coordinates[i].Node < s.coordinates[j].Node
}

// CoordinateDatacenters returns the WAN nodes in each datacenter, along with
// raw network coordinates.
func (s *HTTPServer) CoordinateDatacenters(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, MethodNotAllowedError{req.Method, []string{"GET"}}
	}

	var out []structs.DatacenterMap
	if err := s.agent.RPC("Coordinate.ListDatacenters", struct{}{}, &out); err != nil {
		for i := range out {
			sort.Sort(&sorter{out[i].Coordinates})
		}
		return nil, err
	}

	// Use empty list instead of nil (these aren't really possible because
	// Serf will give back a default coordinate and there's always one DC,
	// but it's better to be explicit about what we want here).
	for i := range out {
		if out[i].Coordinates == nil {
			out[i].Coordinates = make(structs.Coordinates, 0)
		}
	}
	if out == nil {
		out = make([]structs.DatacenterMap, 0)
	}
	return out, nil
}

// CoordinateNodes returns the LAN nodes in the given datacenter, along with
// raw network coordinates.
func (s *HTTPServer) CoordinateNodes(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, MethodNotAllowedError{req.Method, []string{"GET"}}
	}

	args := structs.DCSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var out structs.IndexedCoordinates
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("Coordinate.ListNodes", &args, &out); err != nil {
		sort.Sort(&sorter{out.Coordinates})
		return nil, err
	}

	return filterCoordinates(req, "", out.Coordinates), nil
}

// CoordinateNode returns the LAN node in the given datacenter, along with
// raw network coordinates.
func (s *HTTPServer) CoordinateNode(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, MethodNotAllowedError{req.Method, []string{"GET"}}
	}

	args := structs.DCSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var out structs.IndexedCoordinates
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("Coordinate.ListNodes", &args, &out); err != nil {
		sort.Sort(&sorter{out.Coordinates})
		return nil, err
	}

	node := strings.TrimPrefix(req.URL.Path, "/v1/coordinate/node/")
	return filterCoordinates(req, node, out.Coordinates), nil
}

func filterCoordinates(req *http.Request, node string, in structs.Coordinates) structs.Coordinates {
	out := structs.Coordinates{}

	if in == nil {
		return out
	}

	segment := ""
	v, filterBySegment := req.URL.Query()["segment"]
	if filterBySegment && len(v) > 0 {
		segment = v[0]
	}

	for _, c := range in {
		if node != "" && c.Node != node {
			continue
		}
		if filterBySegment && c.Segment != segment {
			continue
		}
		out = append(out, c)
	}
	return out
}
