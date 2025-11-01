## Overview

The **NavigationAPI** is an AsyncAPI provided by **ACME** that allows developers to access comprehensive navigation data. This includes routes, GPS coordinates, traffic updates, and real-time directions to enhance the user experience in transportation and location-based services.

- **API Name**: NavigationAPI
- **Organization**: ACME
- **API Category**: Transportation
- **API Version**: 3.0.1
- **Tags**: navigation, GPS, routes, traffic
- **API Type**: WS

## API Description

The NavigationAPI provides real-time navigation data and tools for developers to integrate into their applications. Features include:

- Fetching optimized routes based on various criteria (fastest, shortest, etc.)
- Retrieving real-time GPS coordinates and tracking
- Accessing live traffic updates and congestion information
- Supporting geofencing and location-based alerts

## API Endpoints

### Base URLs

- **Sandbox Environment**: `https://sandbox.acme.com/api/v1/navigation`
- **Production Environment**: `https://api.acme.com/v1/navigation`

### Example Endpoints

1. **Get Optimized Route**

   - **Endpoint**: `/routes/optimized`
   - **Method**: `GET`
   - **Description**: Retrieves an optimized route between specified waypoints.
   - **Example Request**: 
   ```bash
   GET https://api.acme.com/v1/navigation/routes/optimized?start=40.712776,-74.005974&end=34.052235,-118.243683
