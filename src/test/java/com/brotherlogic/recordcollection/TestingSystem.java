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

public class TestingSystem implements RcSystem {

  private Logger logger = Logger.getLogger(getClass());
  
  public String getVersion() {
    return "0.1";
  }

  public Config getConfig() {
    return new Config("testkey","testsecret",null);
  }

  public Storage getStorage() {
    return getStorage(false);
  }
  
  public Storage getStorage(boolean clean) {
    //Link to a test database
    try {
      Class.forName("org.postgresql.Driver");
      URI dbUri = new URI(System.getenv("DATABASE_URL"));

      String username = dbUri.getUserInfo().split(":")[0];
      String password = dbUri.getUserInfo().split(":")[1];
      String dbUrl = "jdbc:postgresql://" + dbUri.getHost() + ':' + dbUri.getPort() + dbUri.getPath() + "?ssl=true&sslfactory=org.postgresql.ssl.NonValidatingFactory";

      Connection connection =  DriverManager.getConnection(dbUrl, username, password);

      logger.log(Level.INFO,"Created connection: " + connection);
      
      DatabaseSystem sys = new DatabaseSystem(new DatabaseV1());
      if (clean)
        sys.cleanDatabase(connection);
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
