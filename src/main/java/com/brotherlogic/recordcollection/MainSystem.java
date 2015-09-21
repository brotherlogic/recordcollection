package com.brotherlogic.recordcollection;

import com.brotherlogic.recordcollection.storage.database.DatabaseConnector;
import com.brotherlogic.recordcollection.storage.database.DatabaseStorage;
import com.brotherlogic.recordcollection.storage.database.DatabaseSystem;
import com.brotherlogic.recordcollection.storage.database.DatabaseV2;
import com.brotherlogic.recordcollection.storage.Storage;

import org.apache.log4j.Level;
import org.apache.log4j.Logger;

import java.net.URI;
import java.net.URISyntaxException;
import java.sql.Connection;
import java.sql.DriverManager;
import java.sql.SQLException;

import org.scribe.builder.ServiceBuilder;
import org.scribe.oauth.OAuthService;
import org.scribe.model.Token;

public class MainSystem implements RcSystem {

  private Logger logger = Logger.getLogger(getClass());

  private final String CALLBACK_URL = "http://localhost";

  ConnectionBuilder builder;

  public MainSystem() {
    builder = new DatabaseConnector();
  }

  public MainSystem(ConnectionBuilder build) {
    builder = build;
  }
  
  public String getVersion() {
    return "0.1";
  }

  public Config getConfig() {

    logger.log(Level.INFO,"Building service with " + System.getenv("discogskey") + " and " + System.getenv("discogssecret"));
    
    OAuthService service = new ServiceBuilder()
      .provider(DiscogsApi.class)
      .apiKey(System.getenv("discogskey"))
      .apiSecret(System.getenv("discogssecret"))
      .callback(CALLBACK_URL)
      .build();

    logger.log(Level.INFO,"Built " + service);
    
    return new Config(System.getenv("discogskey"),System.getenv("discogssecret"), (DiscogsService) service);
  }

  public Storage getStorage() {
    return getStorage("org.postgresql.Driver",System.getenv("DATABASE_URL"), new DatabaseSystem(new DatabaseV2()));
  }
  
  protected Storage getStorage(String driverName, String databaseURL, DatabaseSystem sys) {

    if (databaseURL == null)
      return null;
    try {
      Class.forName(driverName);
      logger.log(Level.INFO,"Building from " + databaseURL);
      URI dbUri = new URI(databaseURL);

      if (dbUri.getUserInfo() == null) {
        logger.log(Level.FATAL,databaseURL + " is malformed");
        return null;
      }
      
      String username = dbUri.getUserInfo().split(":")[0];
      String password = dbUri.getUserInfo().split(":")[1];
      String dbUrl = "jdbc:postgresql://" + dbUri.getHost() + ':' + dbUri.getPort() + dbUri.getPath() + "?ssl=true&sslfactory=org.postgresql.ssl.NonValidatingFactory";

      logger.log(Level.FATAL, "database = " + dbUrl);
      
      Connection connection = builder.makeConnection(dbUrl, username, password);

      logger.log(Level.INFO, "Created connection: " + connection);
      sys.initDatabase(connection);
      
      return new DatabaseStorage(connection);
    } catch (SQLException e) {
      logger.log(Level.FATAL,"Cannot connect to database",e);
    } catch (ClassNotFoundException e) {
      logger.log(Level.FATAL,"Cannot find connection class",e);
    } catch (URISyntaxException e) {
      logger.log(Level.FATAL,"Problem with dealing with URI",e);
    }

    logger.log(Level.FATAL, "getStorage returning null");
    return null;
  }
}

