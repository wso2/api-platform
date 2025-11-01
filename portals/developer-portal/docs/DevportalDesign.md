**To customize and theme the developer portal, follow the steps below:**
--------------------

1. Fork and clone the github repository: https://github.com/wso2/api-developer-portal

2. Pre rquisites - Node.


3. Run the following command at the root folder to install the dependencies and start the developer portal.


   ```bash
   npm install
   ```


4. Start the developer portal by executing the following command at the root folder.
  
      ```bash
      npm start
      ``` 
5. Navigate to the organization landing page at [http://localhost:3000](http://localhost:3000/).

6. The following pages can be viewed from below urls:
- [http://localhost:3000](http://localhost:3000/) - Organization landing page
- [http://localhost:3000/apis](http://localhost:3000/apis) - API listing page
- [http://localhost:3000/api/{apiName}](http://localhost:3000/api/{apiName}) - API landing page
- [http://localhost:3000/api/{apiName}/tryout](http://localhost:3000/api/{apiName}/tryout) - API trypout page

7. The project structure is explained below:
  
  - **src**
     - The src folder contains the page layout and content including  the html, images and style sheets.
     - The /src/layout folder includes the main layout of the dev portal. Other pages inherit this layout.
     - The /src/pages folder holds the content for the pages. The html content of each page is in the src/pages directory.
     - Each directory will contain a page.hbs (src/pages/{apis/home/api-landing}/page.hbs) which contains all the partial html content of the page. The partials directory (src/pages/{apis/home/api-landing}/partials) contains the html content for each page (organization landing,api listing, api landing).
     - The /src/partials folder holds the common content for the pages. The header and footer are injected as partials into the layout.
     - The /src/images folder contains the images.  
  - **mock**
    - Includes the mock API information. Mock APIs are displayed to define the structure. In a production scenario, these will be replaced by actual published APIs.
    - Each api folder contains the api definition file, the api metadata and the content to be included in the api landing page (either md or hbs).
    - mock/apiMetadata.json Includes the metadata of the mock APIs to be displayed in the api listing page.
    - mock/auth.json Includes details about the Identity provider to be used when logging in to the dev-portal in development mode.
  **config.json**
  Configuration file for the developer portal.
  **artifacts**
  Contains the sql file to create database tables.

8. Customize the content for the organization landing, api listing and api landing pages.
   From above pages, except for the tryout page, the content can be customized by modifying the hbs files in the src/pages directory.

 **Customize a page**

To customize the content for the api-landing, api-listing or organization landing (home) pages, the relevant hbs files can be modified under the relevant folder for the page in the src/pages directory.

The styles folder (src/styles) will contain the style sheets for all the partials, named after the relevant partial file.

The css and hbs files can be edited and reloaded to view the changes to the relevant pages.

Any new style sheet that is added needs to be imported to the main.css file under the src/styles folder.

The api listing and api landing pages will contain the dynamic data injected and displayed using handlebars.

The dynamic content included for each page is given in the below json structures:

API Listing Page:
```bash
apiMetadata: [
    {
      apiID: '6d9b40b5-46a6-4777-8bde-d72e4660a307',
      apiInfo:{
            "provider": "WSO2",
            "apiName": "NavigationAPI",
            "orgName": "ACME",
            "apiVersion": "3.0.2",
            "apiDescription": "API for retrieving information about hotels and managing reservations",
            "apiType": "REST",
            "visibility": "PRIVATE",
            "visibleGroups": [
                "HR"
            ],
            "owners": {
                "technicalOwner": "john",
                "businessOwner": "sam",
                "businessOwnerEmail": "sam@gmail.com",
                "technicalOwnerEmail": "john@gmail.com"
            },
            "apiImageMetadata": {
                "api-icon": "navigation.jpeg",
                "api-hero": "api.svg"
            }
        },
        "endPoints": {
            "sandboxURL": "string",
            "productionURL": "https://taxi-navigation.mnm.abc.com"
        },
        "subscriptionPolicies": [
            {
                "policyName": "advanced"
            }        
          ]
    }
  ],
  baseUrl: '/ACME'
```
ex:  The apiMetadata can be accessed in the src/pages/apis/partiasl/api-listing.hbs page as show below:

``` bash
{{#apiMetadata}}
 <div class="card dev-card api-card lg-3 d-flex flex-column">
    <img src={{apiInfo.apiImageMetadata.api-icon}} class="api-card-img" alt="..." />
  	<div class="api-card-body flex-grow-1">
    <h5 class="api-card-title">{{apiInfo.apiName}}</h5>
    <p>V {{apiInfo.apiVersion}}</p>
    <p>{{apiInfo.apiType}}</p>
</div>
{{/apiMetadata}}

```

The baseUrl property can be used to create hrefs relative to the base url of the developer portal.

ex:
``` bash
href="{{baseUrl}}/api/{{apiInfo.apiName}}"
```

API Landing Page:
``` bash
{ apiMetadata: {
  apiID: '6d9b40b5-46a6-4777-8bde-d72e4660a307',
  apiInfo: {
    #same as above
  },
  endPoints: {
    sandboxURL: 'https://taxi-navigation.mnm.abc.com',
    productionURL: 'https://taxi-navigation.mnm.abc.com'
  },
  subscriptionPolicies: [
    { policyName: 'advanced' },
    { policyName: 'gold' },
    { policyName: 'platinum' }
  ]
},
baseUrl: '/ACME',
schemaUrl: '[http://localhost:3000/devportal/organizations/{orgID}/apis/{apiID}/template?fileName=apiDefinition.json](http://localhost:3000/devportal/organizations/{orgID}/apis/{apiID}/template?fileName=apiDefinition.json)' }
```
The schemaUrl can be used to download the api definition from the api landing page.

The api information can be accessed from the src/pages/api-landing/partials/api-detail-banner.hbs page as shown below:

``` bash
<div class="api-title">
    <h4>{{apiMetadata.apiInfo.apiName}}</h4>
    <span class="badge api-badge-custom1">{{apiMetadata.apiInfo.apiCategory}}</span>
</div>
<p> {{apiMetadata.apiInfo.apiVersion}}</p>
<p class="api-desciption">{{apiMetadata.apiInfo.apiDescription}}</p>
```
**Customize API landing page**

The mock (/mock) folder will contain each APIs content for the apis displayed in the api listing page (/apis).

An api landing page’s content can either be rendered using a markdown file or by modifying the api-content hbs file directly.

The md file is at /mock/{api-name}/apiContent.md.

The hbs file is at /mock/{api-name}/api-content.hbs

The naming conventions for the md and hbs files are as follows:

markdown: apiContent.md

HBS: api-conent.hbs

**Add a page**
 
In order to add a new page, a folder needs to be created under the pages directory, representing the relevant url path of the page in the directory structure.

  ex: [http://localhost:3000/applications/myapps](http://localhost:3000/applications/myapps)

The relevant partial files and page.hbs need to be added to a folder structure as shown below:
``` bash
 pages
  └─── applications
   └─── myapps
         └── content
             │content.md
         └─ partials
             │appConent.hbs  
         │page.hbs
```

The page.hbs file should have the following format:
``` bash
    {{> appConent }} # name of the partial hbs file
    {{{ content  }}} # name of the markdown file, if any
```
