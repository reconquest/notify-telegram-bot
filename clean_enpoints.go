package main

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	karma "github.com/reconquest/karma-go"
)

func (coordinator *Coordinator) routineCleanEndpoints() error {
	ids, err := coordinator.getUnusedEndpointsIDs()
	if err != nil {
		return karma.Format(
			err,
			"unable to get unused endpoints ids",
		)
	}

	if len(ids) == 0 {
		return nil
	}

	for _, id := range ids {
		err := coordinator.database.RemoveEndpoint(id)
		if err != nil {
			return karma.Format(
				err,
				"unable to remove endpoint with id: %s",
				id.Hex(),
			)
		}
	}

	return nil
}

func (coordinator *Coordinator) getUnusedEndpointsIDs() (
	[]primitive.ObjectID, error) {
	subscribers, err := coordinator.database.FindInSubscriptions(bson.M{})
	if err != nil {
		return nil, karma.Format(
			err,
			"unable to find subscribers",
		)
	}

	endpoints, err := coordinator.database.FindInEndpoints(bson.M{})
	if err != nil {
		return nil, karma.Format(
			err,
			"unable to find endpoints",
		)
	}

	var unusedEndpoints []primitive.ObjectID

	if len(endpoints) == 0 {
		return nil, nil
	}

	if len(subscribers) == 0 {
		for _, endpoint := range endpoints {
			unusedEndpoints = append(unusedEndpoints, endpoint.ID)
		}

		return unusedEndpoints, nil
	}

	var exceptEndpointsIDs []primitive.ObjectID
	for _, subscriber := range subscribers {
		for _, endpoint := range endpoints {
			if subscriber.URL == endpoint.URL &&
				subscriber.Duration == endpoint.Duration {
				exceptEndpointsIDs = append(exceptEndpointsIDs, endpoint.ID)
			}
		}
	}

	var endpointsIDs []primitive.ObjectID
	for _, endpoint := range endpoints {
		endpointsIDs = append(endpointsIDs, endpoint.ID)
	}

	// remove excepted ids from slice
	for _, exceptEndpointsID := range exceptEndpointsIDs {
		for i, endpointsID := range endpointsIDs {
			if endpointsID == exceptEndpointsID {
				endpointsIDs = append(endpointsIDs[:i], endpointsIDs[i+1:]...)
			}
		}
	}

	return endpointsIDs, nil
}
