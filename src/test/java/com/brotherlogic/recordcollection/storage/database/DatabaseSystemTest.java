package com.brotherlogic.recordcollection.storage.database;

import org.junit.Assert;
import org.junit.Test;

import java.sql.Connection;
import java.sql.SQLException;

import org.mockito.Mockito;

public class DatabaseSystemTest {

  @Test
  public void testInitDatabaseWhenNotInited() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);
    Database mDatabase = Mockito.mock(Database.class);
    DatabaseSystem test = new DatabaseSystem(mDatabase);
    Mockito.when(mDatabase.validate(mConnection)).thenReturn(false);
    Mockito.when(mDatabase.getPrevVersion()).thenReturn(null);
    
    test.initDatabase(mConnection);
    Mockito.verify(mDatabase).upgrade(mConnection);
  }
  
  @Test
  public void testInitDatabaseWhenInited() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);
    Database mDatabase = Mockito.mock(Database.class);
    DatabaseSystem test = new DatabaseSystem(mDatabase);
    Mockito.when(mDatabase.validate(mConnection)).thenReturn(true);

    test.initDatabase(mConnection);
    Mockito.verify(mDatabase,Mockito.never()).create(mConnection);
  }

  @Test
  public void testCleanDatabaseWhenNotInited() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);
    Database mDatabase = Mockito.mock(Database.class);
    DatabaseSystem test = new DatabaseSystem(mDatabase);
    Mockito.when(mDatabase.validate(mConnection)).thenReturn(true);

    test.cleanDatabase(mConnection);
    Mockito.verify(mDatabase).destroy(mConnection);
  }

  @Test
  public void testCleanDatabaseWhenInited() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);
    Database mDatabase = Mockito.mock(Database.class);
    DatabaseSystem test = new DatabaseSystem(mDatabase);
    Mockito.when(mDatabase.validate(mConnection)).thenReturn(false);

    test.cleanDatabase(mConnection);
    Mockito.verify(mDatabase,Mockito.never()).destroy(mConnection);
  }

  @Test
  public void testCleanDatabaseWhenFollowOn() throws SQLException {
    Connection mConnection = Mockito.mock(Connection.class);
    Database mDatabase = Mockito.mock(Database.class);
    Database mDatabase2 = Mockito.mock(Database.class);
    Mockito.when(mDatabase.getPrevVersion()).thenReturn(mDatabase2);
    DatabaseSystem test = new DatabaseSystem(mDatabase);
    Mockito.when(mDatabase.validate(mConnection)).thenReturn(false);

    test.cleanDatabase(mConnection);
    Mockito.verify(mDatabase,Mockito.never()).destroy(mConnection);
  }

}
