![Logo](http://svg.wiersma.co.za/glasslabs/module?title=Water&tag=a%02water%20tracking%20module)

Water tracking module for [looking glass](http://github.com/glasslabs/looking-glass)

## Usage

```yaml
modules:
 - name: simple-water
    url:  https://github.com/glasslabs/water/releases/download/v1.0.0/water.wasm
    position: top:right
    config:
      url: http://my-hass-instance:8123
      token: <your-hass-token>
      sensorIds:
        geyserPct: sensor.geyser_hot_water
        tankPct: sensor.reservoir_percentage
      geyser:
        warning: 50
        low: 30
      tank:
        warning: 50
        low: 30
```

## Configuration

### Geyser Percentage Sensor ID (sensorIds.geyserPct)

The Home Assistant geyser percentage sensor ID.

### Tank Percentage Sensor ID (sensorIds.tankPct)

The Home Assistant water tank percentage sensor ID.

### Geyser Warning Percentage (geyser.warning)

The Geyser percentage for the hot water bar to display in warning style.

### Geyser Low Percentage (geyser.low)

The Geyser percentage for the hot water bar to display in low style.

### Tank Warning Percentage (tank.warning)

The Tank percentage for the tank bar to display in warning style.

### Tank Low Percentage (tank.low)

The Tank percentage for the tank bar to display in low style.
