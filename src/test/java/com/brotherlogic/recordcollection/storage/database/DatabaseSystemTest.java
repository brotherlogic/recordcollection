package com.brotherlogic.recordcollection.storage.database;

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

    test.initDatabase(mConnection);
    Mockito.verify(mDatabase).create(mConnection);
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
}
