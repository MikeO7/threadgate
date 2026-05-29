package hass

import "fmt"

// MockDeviceNames returns simulated friendly names for mock-mode dashboard rendering.
func MockDeviceNames() map[string]DeviceDetails {
	m := map[string]DeviceDetails{
		"0000000000000001": {Name: "Living Room Multi-Sensor", Manufacturer: "Eve", Model: "Eve Motion", SwVersion: "1.2.3", Battery: "82", Availability: "on", DeviceID: "dev_eve_motion"},
		"0000000000000002": {Name: "Kitchen Smart Plug", Manufacturer: "Nanoleaf", Model: "Essentials Smart Plug", SwVersion: "3.1.2", Availability: "on", DeviceID: "dev_nano_plug"},
		"0000000000000003": {Name: "Bedroom Radiator Valve", Manufacturer: "Danfoss", Model: "Ally Radiator Thermostat", SwVersion: "2.1.0", Battery: "12", Availability: "on", DeviceID: "dev_dan_valve"},
		"0000000000000004": {Name: "Hallway Motion Detector", Manufacturer: "Philips", Model: "Hue Motion Sensor", SwVersion: "1.5.0", Battery: "95", Availability: "on", DeviceID: "dev_hue_motion"},
		"0000000000000005": {Name: "Office Desk Lamp", Manufacturer: "Ikea", Model: "Tradfri Bulb", SwVersion: "2.0.0", Availability: "off", DeviceID: "dev_ikea_lamp"},
		"0000000000000006": {Name: "Front Door Lock", Manufacturer: "Yale", Model: "Assure Lock 2", SwVersion: "4.2.1", Battery: "45", Availability: "unavailable", DeviceID: "dev_yale_lock"},
		"1122334455667788": {Name: "ThreadGate Border Router", Manufacturer: "ThreadGate", Model: "Border Router Gateway", SwVersion: "1.0.0", Availability: "on", DeviceID: "dev_threadgate"},
	}
	deviceTypes := []string{"Smart Bulb", "Smart Plug", "Thermostat", "Motion Sensor", "Door Sensor", "Window Shade", "Wall Switch", "Light Dimmer"}
	locations := []string{"Kitchen", "Living Room", "Master Bedroom", "Guest Bedroom", "Office", "Hallway", "Basement", "Garage", "Patio", "Attic"}
	for i := 1; i <= 32; i++ {
		mac := fmt.Sprintf("%016x", i)
		if _, ok := m[mac]; !ok {
			loc := locations[(i-1)%len(locations)]
			dtype := deviceTypes[(i-1)%len(deviceTypes)]
			m[mac] = DeviceDetails{
				Name:         fmt.Sprintf("%s %s", loc, dtype),
				Manufacturer: "Nanoleaf",
				Model:        "Essentials " + dtype,
				SwVersion:    "1.0.1",
				Availability: "on",
				DeviceID:     fmt.Sprintf("dev_mock_router_%d", i),
			}
		}
	}
	for i := range 12 {
		mac := fmt.Sprintf("e00000000000%04x", i)
		if _, ok := m[mac]; !ok {
			loc := locations[i%len(locations)]
			m[mac] = DeviceDetails{
				Name:         fmt.Sprintf("%s Sleepy Sensor %d", loc, i+1),
				Manufacturer: "Eve",
				Model:        "Eve Door & Window",
				SwVersion:    "2.0.2",
				Battery:      fmt.Sprintf("%d", (i*8)%100+10),
				Availability: "on",
				DeviceID:     fmt.Sprintf("dev_mock_sleepy_%d", i),
			}
		}
	}
	return m
}
