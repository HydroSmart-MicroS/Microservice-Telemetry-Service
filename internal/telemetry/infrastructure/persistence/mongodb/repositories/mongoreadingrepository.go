package repositories

import (
	"context"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"hydrosmart/telemetry/internal/shared/infrastructure/privacy"
	"hydrosmart/telemetry/internal/telemetry/domain/model/aggregates"
	"hydrosmart/telemetry/internal/telemetry/domain/model/valueobjects"
	"time"
)

type eventDocument struct {
	ID     string `bson:"event_id"`
	Status string `bson:"status"`
}
type readingDocument struct {
	ID              string        `bson:"_id"`
	DeviceID        string        `bson:"device_id"`
	UnitID          string        `bson:"unit_id"`
	BootID          string        `bson:"boot_id"`
	Sequence        int64         `bson:"sequence"`
	VolumeML        int64         `bson:"volume_ml"`
	IntervalSeconds int64         `bson:"interval_seconds"`
	MeasuredAt      time.Time     `bson:"measured_at"`
	ReceivedAt      time.Time     `bson:"received_at"`
	Event           eventDocument `bson:"event"`
}

func document(r aggregates.RecordedReading) readingDocument {
	return readingDocument{r.ID, r.DeviceID, r.UnitID, r.BootID, r.Sequence, r.VolumeML, r.IntervalSeconds, r.MeasuredAt.UTC().Truncate(time.Millisecond), r.ReceivedAt.UTC().Truncate(time.Millisecond), eventDocument{r.ID, "pending"}}
}
func entity(d readingDocument) aggregates.RecordedReading {
	return aggregates.RecordedReading{Reading: aggregates.Reading{ID: d.ID, DeviceID: d.DeviceID, BootID: d.BootID, Sequence: d.Sequence, VolumeML: d.VolumeML, IntervalSeconds: d.IntervalSeconds, MeasuredAt: d.MeasuredAt}, UnitID: d.UnitID, ReceivedAt: d.ReceivedAt}
}

type MongoReadingRepository struct{ Collection *mongo.Collection }

func (r MongoReadingRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.Collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "device_id", Value: 1}, {Key: "boot_id", Value: 1}, {Key: "sequence", Value: 1}}, Options: options.Index().SetUnique(true).SetName("device_boot_sequence_unique")},
		{Keys: bson.D{{Key: "unit_id", Value: 1}, {Key: "measured_at", Value: -1}}, Options: options.Index().SetName("unit_history")},
		{Keys: bson.D{{Key: "device_id", Value: 1}, {Key: "measured_at", Value: -1}}, Options: options.Index().SetName("device_history")},
		{Keys: bson.D{{Key: "event.status", Value: 1}, {Key: "received_at", Value: 1}}, Options: options.Index().SetName("pending_events").SetPartialFilterExpression(bson.M{"event.status": "pending"})},
	})
	return err
}
func (r MongoReadingRepository) Insert(ctx context.Context, reading aggregates.RecordedReading) (bool, error) {
	if err := r.Collection.Database().Collection("erased_units").FindOne(ctx, bson.M{"_id": privacy.Hash(reading.UnitID)}).Err(); err == nil {
		return false, valueobjects.ErrNotFound
	} else if err != mongo.ErrNoDocuments {
		return false, err
	}
	d := document(reading)
	if _, err := r.Collection.InsertOne(ctx, d); err == nil {
		return true, nil
	} else if !mongo.IsDuplicateKeyError(err) {
		return false, err
	}
	var existing readingDocument
	err := r.Collection.FindOne(ctx, bson.M{"$or": bson.A{bson.M{"_id": d.ID}, bson.M{"device_id": d.DeviceID, "boot_id": d.BootID, "sequence": d.Sequence}}}).Decode(&existing)
	if err != nil {
		return false, err
	}
	if existing.DeviceID == d.DeviceID && existing.BootID == d.BootID && existing.Sequence == d.Sequence &&
		existing.UnitID == d.UnitID && existing.VolumeML == d.VolumeML && existing.IntervalSeconds == d.IntervalSeconds && existing.MeasuredAt.Equal(d.MeasuredAt) {
		return false, nil
	}
	return false, valueobjects.ErrConflict
}
func filter(unit string, from, to time.Time) bson.M {
	return bson.M{"unit_id": unit, "measured_at": bson.M{"$gte": from, "$lt": to}}
}
func (r MongoReadingRepository) Summary(ctx context.Context, unit string, from, to time.Time) (aggregates.Consumption, error) {
	var result aggregates.Consumption
	cursor, err := r.Collection.Aggregate(ctx, mongo.Pipeline{
		bson.D{{Key: "$match", Value: filter(unit, from, to)}},
		bson.D{{Key: "$facet", Value: bson.M{
			"total":   bson.A{bson.M{"$group": bson.M{"_id": nil, "volume": bson.M{"$sum": "$volume_ml"}, "count": bson.M{"$sum": 1}, "first": bson.M{"$min": "$measured_at"}, "last": bson.M{"$max": "$measured_at"}}}},
			"daily":   bson.A{bson.M{"$group": bson.M{"_id": bson.M{"date": bson.M{"$dateToString": bson.M{"format": "%Y-%m-%d", "date": "$measured_at", "timezone": "America/Lima"}}, "device": "$device_id"}, "volume": bson.M{"$sum": "$volume_ml"}, "count": bson.M{"$sum": 1}}}, bson.M{"$sort": bson.D{{Key: "_id.date", Value: 1}, {Key: "_id.device", Value: 1}}}},
			"devices": bson.A{bson.M{"$group": bson.M{"_id": "$device_id", "volume": bson.M{"$sum": "$volume_ml"}, "last": bson.M{"$max": "$measured_at"}}}, bson.M{"$sort": bson.M{"_id": 1}}},
		}}},
	})
	if err != nil {
		return result, err
	}
	defer cursor.Close(ctx)
	if cursor.Next(ctx) {
		var row struct {
			Total []struct {
				Volume int64     `bson:"volume"`
				Count  int64     `bson:"count"`
				First  time.Time `bson:"first"`
				Last   time.Time `bson:"last"`
			} `bson:"total"`
			Daily []struct {
				ID struct {
					Date   string `bson:"date"`
					Device string `bson:"device"`
				} `bson:"_id"`
				Volume int64 `bson:"volume"`
				Count  int64 `bson:"count"`
			} `bson:"daily"`
			Devices []struct {
				ID     string    `bson:"_id"`
				Volume int64     `bson:"volume"`
				Last   time.Time `bson:"last"`
			} `bson:"devices"`
		}
		if err = cursor.Decode(&row); err != nil {
			return result, err
		}
		if len(row.Total) > 0 {
			total := row.Total[0]
			result.VolumeML = total.Volume
			result.ReadingCount = total.Count
			result.FirstReadingAt = &total.First
			result.LastReadingAt = &total.Last
		}
		for _, day := range row.Daily {
			result.Daily = append(result.Daily, aggregates.DailyConsumption{Date: day.ID.Date, DeviceID: day.ID.Device, VolumeML: day.Volume, Count: day.Count})
		}
		for _, device := range row.Devices {
			result.Devices = append(result.Devices, aggregates.DeviceConsumption{DeviceID: device.ID, VolumeML: device.Volume, LastReadingAt: device.Last})
		}
	}
	return result, cursor.Err()
}
func (r MongoReadingRepository) History(ctx context.Context, unit string, from, to time.Time, limit int) ([]aggregates.RecordedReading, error) {
	cursor, err := r.Collection.Find(ctx, filter(unit, from, to), options.Find().SetSort(bson.D{{Key: "measured_at", Value: -1}, {Key: "_id", Value: 1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	result := []aggregates.RecordedReading{}
	for cursor.Next(ctx) {
		var row readingDocument
		if err = cursor.Decode(&row); err != nil {
			return nil, err
		}
		result = append(result, entity(row))
	}
	return result, cursor.Err()
}
func (r MongoReadingRepository) Pending(ctx context.Context) ([]aggregates.RecordedReading, error) {
	cursor, err := r.Collection.Find(ctx, bson.M{"event.status": "pending"}, options.Find().SetSort(bson.D{{Key: "received_at", Value: 1}}).SetLimit(100))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	result := []aggregates.RecordedReading{}
	for cursor.Next(ctx) {
		var row readingDocument
		if err = cursor.Decode(&row); err != nil {
			return nil, err
		}
		result = append(result, entity(row))
	}
	return result, cursor.Err()
}
func (r MongoReadingRepository) Published(ctx context.Context, id string) error {
	_, err := r.Collection.UpdateOne(ctx, bson.M{"_id": id, "event.status": "pending"}, bson.M{"$set": bson.M{"event.status": "published", "event.published_at": time.Now().UTC()}})
	return err
}
