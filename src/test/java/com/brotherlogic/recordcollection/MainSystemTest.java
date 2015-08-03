package com.brotherlogic.recordcollection;

import com.brotherlogic.recordcollection.storage.database.DatabaseSystem;
import com.brotherlogic.recordcollection.storage.Storage;

import org.junit.Test;
import org.junit.Assert;

import org.mockito.Mockito;

import java.sql.Connection;

public class MainSystemTest extends BaseTest {

  @Test
  public void testGetVersion() {
    String version = new MainSystem().getVersion();
    Assert.assertTrue(version.length() > 0);
  }
  
  @Test
  public void testGetConfig() {
    Config conf = new MainSystem().getConfig();
    Assert.assertNotNull(conf);
  }

  @Test
  public void testGetStorage() {
    ConnectionBuilder builder = new ConnectionBuilder() {
        public Connection makeConnection(String url, String user, String pass) {
          Connection conn = Mockito.mock(Connection.class);
          return conn;
        }
      };
    DatabaseSystem sys = Mockito.mock(DatabaseSystem.class);
    Storage storage = new MainSystem(builder).getStorage("org.postgresql.Driver","postgres://blah:madeup@blah.blah.com",sys);
    Assert.assertNotNull(storage);
  }

  @Test
  public void testBasicGetStorage() {
    ConnectionBuilder builder = new ConnectionBuilder() {
        public Connection makeConnection(String url, String user, String pass) {
          Connection conn = Mockito.mock(Connection.class);
          return conn;
        }
      };
    
    Storage storage = new MainSystem(builder).getStorage();
    Assert.assertNull(storage);
  }

  @Test
  public void testWithBadURL() {
    Storage storage = new MainSystem().getStorage("org.postgresql.Driver","postgres://blahblahblah", null);
    Assert.assertNull(storage);
  }
  
  @Test
  public void testNullWithMadeUpDB() {
    Storage storage = new MainSystem().getStorage("org.postgresql.Driver", "postgres://blah:madeup@blah.blah.com", null);
    Assert.assertNull(storage);
  }

  @Test
  public void testNullWithBadURI() {
    Storage storage = new MainSystem().getStorage("org.postgresql.Driver", "postgres://blah:what##:madeup@blah.blah.com", null);
    Assert.assertNull(storage);
  }

  @Test
  public void testBadClass() {
    Storage storage = new MainSystem().getStorage("org.postgresql.MadeUpDriver", "postgres://blah:maduep@blah.blah.com", null);
    Assert.assertNull(storage);
  }
  
}
