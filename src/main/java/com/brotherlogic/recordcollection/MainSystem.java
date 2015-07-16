package com.brotherlogic.recordcollection;

import com.brotherlogic.recordcollection.storage.database.DatabaseStorage;
import com.brotherlogic.recordcollection.storage.database.DatabaseSystem;
import com.brotherlogic.recordcollection.storage.database.DatabaseV1;
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
  
  public String getVersion() {
    return "0.1";
  }

  public Config getConfig() {
    OAuthService service = new ServiceBuilder()
      .provider(DiscogsApi.class)
      .apiKey(System.getenv("discogskey"))
      .apiSecret(System.getenv("discogssecret"))
      .callback(CALLBACK_URL)
      .build();

    return new Config(System.getenv("discogskey"),System.getenv("discogssecret"), (DiscogsService) service);
  }

  public Storage getStorage() {
    return getStorage("org.postgresql.Driver",System.getenv("DATABASE_URL"));
  }
  
  protected Storage getStorage(String driverName, String databaseURL) {
    try {
      Class.forName(driverName);
      logger.log(Level.INFO,"Building from " + databaseURL);
      URI dbUri = new URI(databaseURL);

      String username = dbUri.getUserInfo().split(":")[0];
      String password = dbUri.getUserInfo().split(":")[1];
      String dbUrl = "jdbc:postgresql://" + dbUri.getHost() + ':' + dbUri.getPort() + dbUri.getPath() + "?ssl=true&sslfactory=org.postgresql.ssl.NonValidatingFactory";

      Connection connection = DriverManager.getConnection(dbUrl, username, password);

      logger.log(Level.INFO, "Created connection: " + connection);
      
      DatabaseSystem sys = new DatabaseSystem(new DatabaseV1());
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
