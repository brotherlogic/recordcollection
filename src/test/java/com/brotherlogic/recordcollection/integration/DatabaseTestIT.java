package com.brotherlogic.recordcollection.integration;

import com.brotherlogic.recordcollection.RecordCollection;
import com.brotherlogic.recordcollection.TestingSystem;
import com.brotherlogic.recordcollection.storage.database.DatabaseV1;
import com.brotherlogic.recordcollection.storage.Storage;

import org.junit.Assert;
import org.junit.Test;

import org.apache.log4j.Level;
import org.apache.log4j.Logger;

import java.util.Arrays;

public class DatabaseTestIT extends BaseIntegrationTestIT {

  private Logger logger = Logger.getLogger(getClass());
  
  @Test
  public void testDatabaseLoadProcess() throws Exception {
    new TestingSystem().getStorage(true).forceCleanDatabase();
    new TestingSystem(new DatabaseV1()).getStorage(true);
    Storage storage = new TestingSystem().getStorage(false);

    RecordCollection col = new RecordCollection(Arrays.asList(new Integer[] {12,23}), Arrays.asList(new Integer[] {12,45}), "Test");
    logger.log(Level.DEBUG,"HERE = " + storage);
    storage.storeCollection(1234,col);
    RecordCollection col2 = storage.getCollection(1234,"Test");
    Assert.assertEquals(col.getName(),col2.getName());
  }
}
