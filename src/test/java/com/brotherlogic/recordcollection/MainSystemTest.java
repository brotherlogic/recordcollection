package com.brotherlogic.recordcollection;

import com.brotherlogic.recordcollection.storage.Storage;

import org.junit.Test;
import org.junit.Assert;


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
    Storage storage = new MainSystem().getStorage();
    Assert.assertNotNull(storage);
  }

  @Test
  public void testNullWithMadeUpDB() {
    Storage storage = new MainSystem().getStorage("org.postgresql.Driver", "postgres://blah:madeup@blah.blah.com");
    Assert.assertNull(storage);
  }

  @Test
  public void testNullWithBadURI() {
    Storage storage = new MainSystem().getStorage("org.postgresql.Driver", "postgres://blah:what##:madeup@blah.blah.com");
    Assert.assertNull(storage);
  }

  @Test
  public void testBadClass() {
    Storage storage = new MainSystem().getStorage("org.postgresql.MadeUpDriver", System.getenv("DATABASE_URL"));
    Assert.assertNull(storage);
  }
  
}
