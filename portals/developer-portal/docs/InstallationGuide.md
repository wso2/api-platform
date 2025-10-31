Production setup
================

Setting up PostgreSQL
---------------------

### Setting up the database and users

Follow the instructions below to set up the PostgreSQL database and users.

Login to PostgreSQL using a client (e.g. psql). Enter the following command in a command prompt, where USER\_NAME is the username that you will use to access the databases and POSTGRE\_HOST\_IP is the IP of the host of PostgreSQL server.
```bash
psql -h <POSTGRE_HOST_IP> -U <USER_NAME> -W
```

When prompted, specify the password that will be used to access the databases with the username you specified.

In the PostgreSQL command prompt, create the database using the following command:
```bash
postgres# CREATE DATABASE ;
```

Give authorization to the user you use to access the databases as follows.

For example, let's consider devportaladmin as the user.

```bash
postgres# grant all privileges on database to devportaladmin;
```

Log out from the Postgres command prompt by executing the following command:

```bash
postgres# quit;
```

### Executing db scripts to create tables on PostgreSQL database

To create tables for the devportal, execute the relevant script as shown below.

```bash
psql -U \-d \-f /artifacts/script.sql -W
```


### Changing the database to PostgreSQL

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

### Changing the config.json properties

```bash
port = 3000

mode = 'development'

pathToContent = "./src/"
```

port

Default port of the devportal web app

mode

development/production

pathToContent

The path to the src folder, containing the dev-portal pages to load in development mode.

To update the developer portal with the customized content, follow the instructions in the [QuickStart.md].

