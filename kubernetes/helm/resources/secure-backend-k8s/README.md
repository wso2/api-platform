# Secure Backend Kubernetes Deployment

This project contains the Kubernetes configuration files necessary to deploy the secure backend component of the API platform. The secure backend is implemented using NGINX and is designed to handle secure traffic.

## Project Structure

The project has the following files:

- **k8s/deployment.yaml**: Defines the Kubernetes Deployment for the secure backend component, specifying the container image, ports, volume mounts, and health checks.
  
- **k8s/service.yaml**: Defines the Kubernetes Service for the secure backend component, exposing it to other services or external traffic.
  
- **k8s/configmap.yaml**: Defines a ConfigMap to store non-sensitive configuration data for the secure backend, such as NGINX configuration files.
  
- **k8s/secret.yaml**: Defines a Secret to store sensitive information, such as certificates, which are mounted into the secure backend container.

## Deployment Instructions

1. **Prerequisites**: Ensure you have a Kubernetes cluster running and `kubectl` configured to interact with it.

2. **Apply the ConfigMap**: 
   ```bash
   kubectl apply -f k8s/configmap.yaml
   ```

3. **Apply the Secret**: 
   ```bash
   kubectl apply -f k8s/secret.yaml
   ```

4. **Deploy the secure backend**: 
   ```bash
   kubectl apply -f k8s/deployment.yaml
   ```

5. **Expose the secure backend**: 
   ```bash
   kubectl apply -f k8s/service.yaml
   ```

6. **Verify the deployment**: 
   ```bash
   kubectl get pods
   kubectl get services
   ```

## Accessing the Secure Backend

Once deployed, the secure backend can be accessed through the service defined in `k8s/service.yaml`. Ensure that the appropriate ports are open and that you have the necessary certificates for secure communication.

## Additional Notes

- Ensure that the NGINX configuration file is correctly set up in the ConfigMap.
- Certificates should be stored securely in the Secret and mounted properly in the deployment.