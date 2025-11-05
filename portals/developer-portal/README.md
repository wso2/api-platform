**Pre-requisites**  
Install Node.js v22.0.0
Install POSTGRESQL and create a database named 'devportal'.  
For more information, refer to docs/InstallationGuide.md.

**Changing the database to PostgreSQL**

1.  Navigate to the config.json file in the <DEVPORTAL_HOME> directory.
    
2.  Edit the db properties to connect to the created db.
    
```bash
db = {

    username: 'postgres',

    password: 'postgres',

    database: 'devportal',

    host: 'localhost',

    dialect: 'postgres'
}
```

**Try Out Devportal**  

1.  Run the following command to create a default organization and populate the developer portal landing page content.
    ```bash
    ./artifacts/org_data.sh
    ```

2.  Run the following command to create mock APIs and populate api landing page content.

    ```bash
    ./artifacts/api_data.sh
    ```
    To delete the mock APIs, run the following command.
    ```bash
    ./artifacts/delete_api_data.sh
    ```
3. Run the following command at the root folder to install the dependencies and start the developer portal.

    ```bash
    npm install
    ```
4.  Run the following command to start the developer portal.:
     
    ```bash
    npm start
    ```

    The default organization name is ACME.
    This will start the webapp in the following URL : ‘[http://localhost:3000/ACME](http://localhost:3000/{orgName})’

    This will direct you to the organization landing page.

    The http://localhost:3000/ACME/apis will direct you to the api listing page.

    Each of the api landing and tryout pages are available at the following URLs:
      'http://localhost:3000/ACME/api/{apiName}'
      'http://localhost:3000/ACME/api/{apiName}/tryout'


**Add a new third party API to the developer portal**

- Configure a control plane in the developer portal.
 ```bash
 curl --location 'http://localhost:3000/devportal/organizations/{{orgId}}/provider' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer <<access_token>>' \
--data '{
    "name": "MuleSoft",
    "providerURL": "https://anypoint.mulesoft.com/login/signin?apintent=generic"
}'
 ```


-  Create an API in the developer portal:
    
The apiType values include REST, AsyncAPI, GraphQL or SOAP

The provider value should be the name of the control plane configured in the developer portal.

visibility - PUBLIC/PRIVATE

This is a multi part request containing a json with metadata related to the API and a file attachement of the api schema definition file.
``` bash
 curl --location 'http://localhost:devportal/organizations/{organizationID}/apis'

   --form 'api-metadata="{
                  "apiInfo": { 
                     "referenceID": "<UUID for the API created in WSO2 publisher>",
                     "apiName": "NavigationAPI",
                     "provider": "MuleSoft",
                     "orgName": "ACME",
                     "apiCategory": "Travel",
                     "apiDescription": "API for retrieving information about hotels and managing reservations",
                     "visibility" : "PUBLIC",
                     "visibleGroups": \["HR"\],
                     "owners" : {
                       "technicalOwner": "john",
                       "technicalOwnerEmail": "john@gmail.com",
                       "businessOwner" : "sam",
                       "businessOwnerEmail":"sam@gmail.com"
                     },
                     "apiVersion" : "3.0.2",
                     "apiType" : "REST"
                  },
                  "subscriptionPolicies": [
                     {
                        "policyName": "gold"
                     },
                     {
                        "policyName": "platinum"
                     }
                  ],
                  "endPoints": {
                     "sandboxURL": "http://taxi-navigation.mnm.abc.com",
                     "productionURL": "https://taxi-navigation.mnm.abc.com"
                  }
            }"; type=application/json'
   --form 'apiDefinition=@"{apiDefinition.json}"'
```
- Upload the api landing page content.

To upload the content to be displayed on the api-landing page, create a zip file with the folder structure as follows:
``` bash
{API NAME}
└───content
   │   api-content.hbs
   │   apiContent.md
└───images
   │   icon.svg
   │   product.png
```
This is a multi part request, containing the metadata about the images in the api landing page.   
This should be sent as a json key value map, with the key referring to the name in the hbs where the image is referenced and the name of the image file as the value.  
For example, the key should be sent as api-icon to display the relevant image in the following img tag.
``` bash
    <img src={{apiInfo.apiImageMetadata.api-icon}} class="api-card-img" alt="..." />

```
Run the following command to upload the content.
``` bash
curl --location --request POST 'http://localhost:3000/devportal/organizations/{organizationID}/apis/{apiID}/template'
     --form 'apiContent=@"{path-to-zip-file}"' \\
     --form 'imageMetadata="{
               \\"api-icon\\" : \\"navigation.jpeg\\",
               \\"api-hero\\": \\"api.svg\\"
            }"
```

Use this (https://devportal-4432.postman.co/workspace/Devportal-Workspace~9221a728-2c4b-46ec-acc3-095b9debacbc/collection/5029047-61d763dc-d7b9-4436-9a2e-94585c806943?action=share&creator=5029047) POSTMAN collection to test the API requests.
